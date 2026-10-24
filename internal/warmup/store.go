package warmup

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gofrs/flock"
	_ "github.com/mattn/go-sqlite3"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var ErrConflict = errors.New("configuration changed; reload before saving")

type Store struct {
	mu   sync.Mutex
	db   *sql.DB
	lock *flock.Flock
	path string
}

func Open(dir string) (*Store, error) {
	abs, e := filepath.Abs(dir)
	if e != nil {
		return nil, e
	}
	if e = os.MkdirAll(abs, 0700); e != nil {
		return nil, e
	}
	lock := flock.New(filepath.Join(abs, "instance.lock"))
	ok, e := lock.TryLock()
	if e != nil || !ok {
		return nil, errors.New("state directory is in use")
	}
	path := filepath.Join(abs, "warmup.db")
	file, e := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		lock.Unlock()
		return nil, e
	}
	file.Close()
	dsn := sqliteFileURI(path)
	db, e := sql.Open("sqlite3", dsn+"?_busy_timeout=5000&_journal_mode=WAL&_synchronous=FULL&_foreign_keys=on")
	if e != nil {
		lock.Unlock()
		return nil, e
	}
	db.SetMaxOpenConns(1)
	var schema int
	if e = db.QueryRow("PRAGMA user_version").Scan(&schema); e != nil {
		db.Close()
		lock.Unlock()
		return nil, fmt.Errorf("read database schema: %w", e)
	}
	if schema > 1 {
		db.Close()
		lock.Unlock()
		return nil, fmt.Errorf("unsupported database schema version: %d", schema)
	}
	_, e = db.Exec(`CREATE TABLE IF NOT EXISTS config(id INTEGER PRIMARY KEY CHECK(id=1), body BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS executions(id TEXT PRIMARY KEY, dedup TEXT NOT NULL UNIQUE, body BLOB NOT NULL);
 CREATE TABLE IF NOT EXISTS runs(id TEXT PRIMARY KEY, task_id TEXT NOT NULL, name TEXT NOT NULL, model TEXT NOT NULL, provider TEXT NOT NULL, mode TEXT NOT NULL, started INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS execution_keys(dedup TEXT PRIMARY KEY);
 CREATE TABLE IF NOT EXISTS claims(key TEXT PRIMARY KEY, run_id TEXT NOT NULL);
 PRAGMA user_version=1;`)
	if e != nil {
		db.Close()
		lock.Unlock()
		return nil, e
	}
	s := &Store{db: db, lock: lock, path: abs}
	if e = s.initHistoryIndex(); e != nil {
		s.Close()
		return nil, e
	}
	_, e = db.Exec("INSERT OR IGNORE INTO config VALUES(1,?)", marshal(DefaultConfig()))
	if e != nil {
		s.Close()
		return nil, e
	}
	_ = os.Chmod(path, 0600)
	config, e := s.Config()
	if e != nil {
		s.Close()
		return nil, e
	}
	if e = Validate(&config); e != nil {
		s.Close()
		return nil, e
	}
	// Persist uncertain in-flight results before the scheduler can start. Never replay them.
	xs, e := s.Executions()
	if e != nil {
		s.Close()
		return nil, e
	}
	for _, x := range xs {
		if x.Status == "running" || x.Status == "queued" {
			if x.Status == "running" {
				x.Status = "unknown"
				x.Message = "Process interrupted; result unknown. No retry."
			} else {
				x.Status = "skipped"
				x.Message = "Restart: pending requests were not replayed."
			}
			x.Finished = time.Now().UnixMilli()
			if e = s.Put(x); e != nil {
				s.Close()
				return nil, e
			}
		}
	}
	return s, nil
}

func sqliteFileURI(path string) string {
	uriPath := filepath.ToSlash(path)
	// A drive letter belongs in the path of file:///C:/..., never its authority.
	if !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	return (&url.URL{Scheme: "file", Path: uriPath}).String()
}

func (s *Store) Close() error { e := s.db.Close(); _ = s.lock.Unlock(); return e }
func (s *Store) Config() (Config, error) {
	var raw []byte
	c := DefaultConfig()
	e := s.db.QueryRow("SELECT body FROM config WHERE id=1").Scan(&raw)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(raw, &c)
	return c, e
}
func (s *Store) Save(c Config) error {
	if e := Validate(&c); e != nil {
		return e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old, e := s.Config()
	if e != nil {
		return e
	}
	if old.Revision != c.Revision {
		return ErrConflict
	}
	c.Revision++
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec("UPDATE config SET body=? WHERE id=1", marshal(c)); e != nil {
		return e
	}
	// Paused/deleted scheduled sources are withdrawn; manual runs are explicit independent actions.
	rows, e := tx.Query("SELECT body FROM executions WHERE json_extract(body,'$.status')='queued'")
	if e != nil {
		return e
	}
	xs := []Execution{}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			rows.Close()
			return e
		}
		var x Execution
		if e = json.Unmarshal(b, &x); e != nil {
			rows.Close()
			return e
		}
		xs = append(xs, x)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, x := range xs {
		if x.Status != "queued" {
			continue
		}
		reason := ""
		if c.Settings.Paused {
			reason = "Global pause"
		} else if c.Overrides[x.AuthID] {
			reason = "Account paused"
		} else if x.Mode == "scheduled" {
			valid := false
			for i := range x.Origins {
				if !x.Origins[i].Active {
					continue
				}
				found := false
				for _, t := range c.Tasks {
					if t.ID == x.Origins[i].TaskID && t.Enabled {
						found = true
					}
				}
				if !found {
					x.Origins[i].Active = false
				} else {
					valid = true
				}
			}
			if !valid {
				reason = "All scheduled task sources paused or removed"
			}
		}
		if reason != "" {
			x.Status = "skipped"
			x.Message = reason
			x.Finished = time.Now().UnixMilli()
		}
		if _, e = tx.Exec("UPDATE executions SET body=? WHERE id=?", marshal(x), x.ID); e != nil {
			return e
		}
	}
	return tx.Commit()
}
func (s *Store) Executions() ([]Execution, error) {
	return s.executionQuery("SELECT body FROM executions ORDER BY rowid")
}
func (s *Store) QueuedExecutions() ([]Execution, error) {
	return s.executionQuery("SELECT body FROM executions WHERE json_extract(body,'$.status')='queued' ORDER BY rowid")
}
func (s *Store) executionQuery(query string) ([]Execution, error) {
	rows, e := s.db.Query(query)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	xs := []Execution{}
	for rows.Next() {
		var b []byte
		var x Execution
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		if e = json.Unmarshal(b, &x); e != nil {
			return nil, e
		}
		xs = append(xs, x)
	}
	return xs, rows.Err()
}
func (s *Store) Put(x Execution) error {
	_, e := s.db.Exec("UPDATE executions SET body=? WHERE id=?", marshal(x), x.ID)
	return e
}

// Prune display history only; claims and compact execution keys survive cleanup.
func (s *Store) Prune(now time.Time, days int) error {
	xs, err := s.Executions()
	if err != nil {
		return err
	}
	cutoff := now.AddDate(0, 0, -days).UnixMilli()
	activeRuns := map[string]bool{}
	for _, x := range xs {
		if x.Status == "queued" || x.Status == "running" || x.Finished >= cutoff {
			for _, o := range x.Origins {
				activeRuns[o.RunID] = true
			}
		}
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.Query("SELECT id,started FROM runs")
	if err != nil {
		return err
	}
	ids := []string{}
	keep := map[string]bool{}
	for rows.Next() {
		var id string
		var at int64
		if err = rows.Scan(&id, &at); err != nil {
			rows.Close()
			return err
		}
		if at < cutoff && !activeRuns[id] {
			ids = append(ids, id)
		} else {
			keep[id] = true
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err = tx.Exec("DELETE FROM runs WHERE id=?", id); err != nil {
			return err
		}
	}
	for _, x := range xs {
		if x.Status == "queued" || x.Status == "running" || x.Finished >= cutoff {
			continue
		}
		visible := false
		for _, o := range x.Origins {
			visible = visible || keep[o.RunID]
		}
		if visible {
			continue
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO execution_keys SELECT dedup FROM executions WHERE id=?", x.ID); err != nil {
			return err
		}
		if _, err = tx.Exec("DELETE FROM executions WHERE id=?", x.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

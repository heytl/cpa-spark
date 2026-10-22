package warmup

import (
	"errors"
	"strings"
	"time"
)

type HistoryFilter struct {
	Query    string `json:"query"`
	Date     string `json:"date"`
	Status   string `json:"status"`
	Timezone string `json:"timezone"`
}
type HistoryDelete struct {
	IDs     []string       `json:"ids"`
	All     bool           `json:"all"`
	Filter  *HistoryFilter `json:"filter"`
	Preview bool           `json:"preview"`
}

// DeleteHistory removes only display records. Claims and execution deduplication
// are retained, so deleting a record can never replay a charged request.
func (e *Engine) DeleteHistory(req HistoryDelete) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	modes := 0
	if req.All {
		modes++
	}
	if req.Filter != nil {
		modes++
	}
	if len(req.IDs) > 0 {
		modes++
	}
	if modes != 1 || len(req.IDs) > 5000 {
		return 0, errors.New("select exactly one history deletion scope")
	}
	zone := time.UTC
	if f := req.Filter; f != nil {
		var err error
		zone, err = time.LoadLocation(f.Timezone)
		if err != nil {
			return 0, errors.New("invalid history timezone")
		}
		if f.Date != "" {
			if _, err = time.Parse("2006-01-02", f.Date); err != nil {
				return 0, errors.New("invalid history date")
			}
		}
		switch f.Status {
		case "", "all", "success", "failed", "partial", "skipped", "unknown", "running":
		default:
			return 0, errors.New("invalid history status")
		}
	}
	xs, err := e.store.Executions()
	if err != nil {
		return 0, err
	}
	statuses := map[string][]string{}
	for _, x := range xs {
		for _, o := range x.Origins {
			status := x.Status
			if !o.Active {
				status = "skipped"
			}
			statuses[o.RunID] = append(statuses[o.RunID], status)
		}
	}
	requested := map[string]bool{}
	for _, id := range req.IDs {
		requested[id] = true
	}
	rows, err := e.store.db.Query("SELECT id,name,model,started FROM runs")
	if err != nil {
		return 0, err
	}
	ids := []string{}
	for rows.Next() {
		var id, name, model string
		var started int64
		if err = rows.Scan(&id, &name, &model, &started); err != nil {
			rows.Close()
			return 0, err
		}
		status := "success"
		unknown, failed, skipped, running := false, false, 0, false
		for _, s := range statuses[id] {
			unknown = unknown || s == "unknown"
			failed = failed || s == "failed"
			running = running || s == "queued" || s == "running"
			if s == "skipped" {
				skipped++
			}
		}
		if running {
			continue
		}
		if unknown {
			status = "unknown"
		} else if failed {
			status = "failed"
		} else if len(statuses[id]) == 0 || skipped == len(statuses[id]) {
			status = "skipped"
		} else if skipped > 0 {
			status = "partial"
		}
		if !req.All && req.Filter == nil && !requested[id] {
			continue
		}
		if f := req.Filter; f != nil {
			if f.Status != "" && f.Status != "all" && f.Status != status {
				continue
			}
			if f.Date != "" && time.UnixMilli(started).In(zone).Format("2006-01-02") != f.Date {
				continue
			}
			if f.Query != "" && !strings.Contains(strings.ToLower(name+" "+model+" "+id), strings.ToLower(f.Query)) {
				continue
			}
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if req.Preview {
		return len(ids), nil
	}
	tx, err := e.store.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for _, id := range ids {
		if _, err = tx.Exec("DELETE FROM runs WHERE id=?", id); err != nil {
			return 0, err
		}
	}
	return len(ids), tx.Commit()
}

// RunOnce uses an explicit draft without adding a scheduled task to config.
func (e *Engine) RunOnce(t Task, key string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped || e.lastError != "" {
		return "", errors.New("scheduler unavailable")
	}
	if len(key) < 8 || len(key) > 128 {
		return "", errors.New("idempotency key required (8-128 characters)")
	}
	c, err := e.store.Config()
	if err != nil {
		return "", err
	}
	if c.Settings.Paused {
		return "", errors.New("global schedule paused")
	}
	t.ID = "once-" + key
	t.Enabled = true
	t.Revision = 1
	t.Times = []string{"00:00"}
	t.Days = []int{0, 1, 2, 3, 4, 5, 6}
	if t.Timezone == "" {
		t.Timezone = c.Settings.Timezone
	}
	check := c
	check.Tasks = []Task{t}
	if err = Validate(&check); err != nil {
		return "", err
	}
	ids, err := e.enqueue([]Task{t}, c, e.now(), "manual", key)
	if err != nil {
		return "", err
	}
	return ids[0], nil
}

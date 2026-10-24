package warmup

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type HistoryQuery struct {
	Mode     string        `json:"mode"`
	Filter   HistoryFilter `json:"filter"`
	Page     int           `json:"page"`
	PageSize int           `json:"pageSize"`
}

const historyTaskSQL = `SELECT r.id,r.task_id taskId,r.name,r.model,r.provider,r.mode,r.started,count(x.id) accountCount,
 CASE WHEN sum(CASE WHEN o.active AND json_extract(x.body,'$.status') IN ('queued','running') THEN 1 ELSE 0 END)>0 THEN 'running'
 WHEN sum(CASE WHEN o.active AND json_extract(x.body,'$.status')='unknown' THEN 1 ELSE 0 END)>0 THEN 'unknown'
 WHEN sum(CASE WHEN o.active AND json_extract(x.body,'$.status')='failed' THEN 1 ELSE 0 END)>0 THEN 'failed'
 WHEN count(x.id)=0 OR sum(CASE WHEN NOT o.active OR json_extract(x.body,'$.status')='skipped' THEN 1 ELSE 0 END)=count(x.id) THEN 'skipped'
 WHEN sum(CASE WHEN NOT o.active OR json_extract(x.body,'$.status')='skipped' THEN 1 ELSE 0 END)>0 THEN 'partial' ELSE 'success' END status
 FROM runs r LEFT JOIN history_origins o ON o.run_id=r.id LEFT JOIN executions x ON x.id=o.execution_id GROUP BY r.id`
const historyAccountSQL = `SELECT x.id,coalesce(json_extract(x.body,'$.authId'),'') authId,json_extract(x.body,'$.model') model,json_extract(x.body,'$.mode') mode,
 coalesce(json_extract(x.body,'$.status'),'unknown') status,
 coalesce(nullif(json_extract(x.body,'$.started'),0),nullif(json_extract(x.body,'$.scheduled'),0),(SELECT min(r.started) FROM history_origins o JOIN runs r ON r.id=o.run_id WHERE o.execution_id=x.id),0) started
 FROM executions x WHERE EXISTS(SELECT 1 FROM history_origins o JOIN runs r ON r.id=o.run_id WHERE o.execution_id=x.id)`

func historyWhere(mode string, f HistoryFilter) (string, []any, error) {
	if mode != "account" && mode != "task" {
		return "", nil, errors.New("invalid history mode")
	}
	if len(f.Query) > 500 || len(f.TaskID) > 200 || len(f.AccountMatches) > 5000 {
		return "", nil, errors.New("history filter too large")
	}
	switch f.Status {
	case "", "all", "success", "failed", "partial", "skipped", "unknown", "running", "queued":
	default:
		return "", nil, errors.New("invalid history status")
	}
	zone := time.UTC
	if f.Timezone != "" {
		var err error
		zone, err = time.LoadLocation(f.Timezone)
		if err != nil {
			return "", nil, errors.New("invalid history timezone")
		}
	}
	clauses := []string{"1=1"}
	args := []any{}
	if f.Date != "" {
		day, err := time.ParseInLocation("2006-01-02", f.Date, zone)
		if err != nil {
			return "", nil, errors.New("invalid history date")
		}
		clauses = append(clauses, "h.started>=? AND h.started<?")
		args = append(args, day.UnixMilli(), day.AddDate(0, 0, 1).UnixMilli())
	}
	if f.Status != "" && f.Status != "all" {
		clauses = append(clauses, "h.status=?")
		args = append(args, f.Status)
	}
	if f.TaskID != "" {
		if mode == "task" {
			clauses = append(clauses, "h.taskId=?")
		} else {
			clauses = append(clauses, "EXISTS(SELECT 1 FROM history_origins o JOIN runs r ON r.id=o.run_id WHERE o.execution_id=h.id AND r.task_id=?)")
		}
		args = append(args, f.TaskID)
	}
	if f.Query != "" {
		if mode == "task" {
			clauses = append(clauses, "instr(lower(h.name||' '||h.model||' '||h.id),lower(?))>0")
			args = append(args, f.Query)
		} else {
			part := "(instr(lower(h.authId||' '||coalesce(h.model,'')||' '||h.id),lower(?))>0 OR EXISTS(SELECT 1 FROM history_origins o JOIN runs r ON r.id=o.run_id WHERE o.execution_id=h.id AND instr(lower(r.name||' '||r.id),lower(?))>0)"
			args = append(args, f.Query, f.Query)
			if len(f.AccountMatches) > 0 {
				part += " OR h.authId IN (" + strings.TrimRight(strings.Repeat("?,", len(f.AccountMatches)), ",") + ")"
				for _, id := range f.AccountMatches {
					args = append(args, id)
				}
			}
			clauses = append(clauses, part+")")
		}
	}
	return strings.Join(clauses, " AND "), args, nil
}
func historyBase(mode string) string {
	if mode == "task" {
		return historyTaskSQL
	}
	return historyAccountSQL
}

// History only decodes the requested page. Filtering and counts run in SQLite.
func (e *Engine) History(q HistoryQuery) (map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if q.Mode == "" {
		q.Mode = "account"
	}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = 50
	}
	if q.Page < 1 || (q.PageSize != 20 && q.PageSize != 50 && q.PageSize != 100) {
		return nil, errors.New("invalid history page")
	}
	where, args, err := historyWhere(q.Mode, q.Filter)
	if err != nil {
		return nil, err
	}
	base := "WITH h AS (" + historyBase(q.Mode) + ") "
	var total, deletable, all int
	if err = e.store.db.QueryRow(base+"SELECT count(*),coalesce(sum(status NOT IN ('queued','running')),0) FROM h WHERE "+where, args...).Scan(&total, &deletable); err != nil {
		return nil, err
	}
	if err = e.store.db.QueryRow(base + "SELECT count(*) FROM h WHERE status NOT IN ('queued','running')").Scan(&all); err != nil {
		return nil, err
	}
	pages := (total + q.PageSize - 1) / q.PageSize
	if pages < 1 {
		pages = 1
	}
	if q.Page > pages {
		q.Page = pages
	}
	rows, err := e.store.db.Query(base+"SELECT * FROM h WHERE "+where+" ORDER BY started DESC,id DESC LIMIT ? OFFSET ?", append(args, q.PageSize, (q.Page-1)*q.PageSize)...)
	if err != nil {
		return nil, err
	}
	cols, err := rows.Columns()
	if err != nil {
		rows.Close()
		return nil, err
	}
	records := []map[string]any{}
	for rows.Next() {
		values := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range values {
			dest[i] = &values[i]
		}
		if err = rows.Scan(dest...); err != nil {
			rows.Close()
			return nil, err
		}
		record := map[string]any{}
		for i, key := range cols {
			value := values[i]
			if b, ok := value.([]byte); ok {
				value = string(b)
			}
			record[key] = value
		}
		record["finished"] = record["status"] != "running" && record["status"] != "queued"
		records = append(records, record)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if q.Mode == "account" {
		for _, record := range records {
			sources, err := e.store.db.Query(`SELECT r.id,r.task_id,r.name FROM history_origins o JOIN runs r ON r.id=o.run_id WHERE o.execution_id=? ORDER BY r.started DESC,r.id DESC`, record["id"])
			if err != nil {
				return nil, err
			}
			runs := []map[string]any{}
			for sources.Next() {
				var id, taskID, name string
				if err = sources.Scan(&id, &taskID, &name); err != nil {
					sources.Close()
					return nil, err
				}
				runs = append(runs, map[string]any{"id": id, "taskId": taskID, "name": name})
			}
			err = sources.Err()
			sources.Close()
			if err != nil {
				return nil, err
			}
			record["runs"] = runs
			if len(runs) > 0 {
				record["runId"] = runs[0]["id"]
			}
		}
	}
	return map[string]any{"rows": records, "total": total, "deletable": deletable, "allDeletable": all, "page": q.Page, "pageSize": q.PageSize, "pages": pages}, nil
}

func (s *Store) runDetail(id string, accountID ...string) (map[string]any, error) {
	var tid, name, model, provider, mode string
	var at int64
	if err := s.db.QueryRow("SELECT task_id,name,model,provider,mode,started FROM runs WHERE id=?", id).Scan(&tid, &name, &model, &provider, &mode, &at); err != nil {
		return nil, err
	}
	query := `SELECT x.body,o.active FROM history_origins o JOIN executions x ON x.id=o.execution_id WHERE o.run_id=?`
	args := []any{id}
	if len(accountID) > 0 {
		query += " AND json_extract(x.body,'$.authId')=?"
		args = append(args, accountID[0])
	}
	query += " ORDER BY x.rowid"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	finished := true
	for rows.Next() {
		var raw []byte
		var active bool
		var x Execution
		if err = rows.Scan(&raw, &active); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &x); err != nil {
			return nil, err
		}
		status, message := x.Status, x.Message
		if !active {
			status = "skipped"
			message = "Source cancelled before dispatch"
		}
		if status == "queued" || status == "running" {
			finished = false
		}
		names := []string{}
		for _, o := range x.Origins {
			if o.Active {
				names = append(names, o.Name)
			}
		}
		items = append(items, map[string]any{"id": x.ID, "authId": x.AuthID, "executionStatus": x.Status, "status": status, "message": message, "latency": x.Latency, "tokens": x.Tokens, "sources": names, "httpStatus": x.HTTPStatus, "started": x.Started, "finished": x.Finished, "scheduled": x.Scheduled})
	}
	return map[string]any{"id": id, "taskId": tid, "name": name, "model": model, "provider": provider, "mode": mode, "started": at, "finished": finished, "items": items}, rows.Err()
}
func (e *Engine) HistoryDetail(id string) (map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.store.runDetail(id)
}

func (e *Engine) AccountTimeline(id string) (map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	cutoff := e.now().Add(-24 * time.Hour).UnixMilli()
	// One row per actual execution, irrespective of merged task sources. Only the
	// selected account's bodies are decoded; details still load by real run ID.
	rows, err := e.store.db.Query(`WITH selected AS (SELECT x.body,(SELECT o.run_id FROM history_origins o JOIN runs r ON r.id=o.run_id WHERE o.execution_id=x.id ORDER BY o.active DESC,r.started DESC,r.id DESC LIMIT 1) run_id FROM executions x WHERE json_extract(x.body,'$.authId')=? AND (json_extract(x.body,'$.finished')>? OR json_extract(x.body,'$.started')>?)) SELECT x.body,r.id,r.task_id,r.name,r.model,r.provider,r.mode,r.started FROM selected x JOIN runs r ON r.id=x.run_id ORDER BY r.started DESC,r.id DESC`, id, cutoff, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	runs := []map[string]any{}
	for rows.Next() {
		var raw []byte
		var rid, tid, name, model, provider, mode string
		var at int64
		if err = rows.Scan(&raw, &rid, &tid, &name, &model, &provider, &mode, &at); err != nil {
			return nil, err
		}
		var x Execution
		if err = json.Unmarshal(raw, &x); err != nil {
			return nil, err
		}
		item := map[string]any{"id": x.ID, "authId": x.AuthID, "status": x.Status, "executionStatus": x.Status, "message": x.Message, "started": x.Started, "finished": x.Finished, "scheduled": x.Scheduled}
		runs = append(runs, map[string]any{"id": rid, "taskId": tid, "name": name, "model": model, "provider": provider, "mode": mode, "started": at, "finished": x.Status != "queued" && x.Status != "running", "items": []map[string]any{item}})
	}
	return map[string]any{"runs": runs}, rows.Err()
}

// The transaction re-evaluates the same full-dataset filter used by the list.
func (e *Engine) deleteHistorySQL(req HistoryDelete) (int, error) {
	mode := req.Mode
	if mode == "" {
		mode = "task"
	}
	if mode != "task" && mode != "account" {
		return 0, errors.New("invalid history mode")
	}
	scopes := 0
	for _, set := range []bool{req.All, req.Filter != nil, len(req.IDs) > 0, len(req.ExecutionIDs) > 0} {
		if set {
			scopes++
		}
	}
	if scopes != 1 || len(req.IDs) > 5000 || len(req.ExecutionIDs) > 5000 {
		return 0, errors.New("select exactly one history deletion scope")
	}
	if mode == "account" && len(req.IDs) > 0 || mode == "task" && len(req.ExecutionIDs) > 0 {
		return 0, errors.New("history IDs do not match mode")
	}
	filter := HistoryFilter{}
	if req.Filter != nil {
		filter = *req.Filter
	}
	where, args, err := historyWhere(mode, filter)
	if err != nil {
		return 0, err
	}
	where += " AND h.status NOT IN ('running','queued')"
	ids := req.IDs
	if mode == "account" {
		ids = req.ExecutionIDs
	}
	if len(ids) > 0 {
		where += " AND h.id IN (" + strings.TrimRight(strings.Repeat("?,", len(ids)), ",") + ")"
		for _, id := range ids {
			args = append(args, id)
		}
	}
	tx, err := e.store.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("CREATE TEMP TABLE IF NOT EXISTS history_delete_ids(id TEXT PRIMARY KEY); DELETE FROM history_delete_ids"); err != nil {
		return 0, err
	}
	if _, err = tx.Exec("WITH h AS ("+historyBase(mode)+") INSERT INTO history_delete_ids SELECT id FROM h WHERE "+where, args...); err != nil {
		return 0, err
	}
	var count int
	if err = tx.QueryRow("SELECT count(*) FROM history_delete_ids").Scan(&count); err != nil {
		return 0, err
	}
	if req.Preview {
		return count, nil
	}
	if mode == "account" {
		_, err = tx.Exec(`INSERT OR IGNORE INTO execution_keys SELECT dedup FROM executions WHERE id IN (SELECT id FROM history_delete_ids);
  DELETE FROM runs WHERE id IN (SELECT run_id FROM history_origins WHERE execution_id IN (SELECT id FROM history_delete_ids)) AND NOT EXISTS(SELECT 1 FROM history_origins o WHERE o.run_id=runs.id AND o.execution_id NOT IN(SELECT id FROM history_delete_ids));
  DELETE FROM executions WHERE id IN(SELECT id FROM history_delete_ids);`)
	} else {
		_, err = tx.Exec("DELETE FROM runs WHERE id IN(SELECT id FROM history_delete_ids)")
	}
	if err != nil {
		return 0, err
	}
	return count, tx.Commit()
}

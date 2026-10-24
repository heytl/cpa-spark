package warmup

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Engine struct {
	mu             sync.Mutex
	store          *Store
	host           Host
	now            func() time.Time
	stopped        bool
	stop           chan struct{}
	done           chan struct{}
	workers        sync.WaitGroup
	active         map[string]string
	providerActive map[string]int
	cursor         time.Time
	lastError      string
	lastPruned     time.Time
}

func NewEngine(s *Store, h Host, now func() time.Time) *Engine {
	return &Engine{store: s, host: h, now: now, stop: make(chan struct{}), done: make(chan struct{}), active: map[string]string{}, providerActive: map[string]int{}, cursor: now().Truncate(time.Minute)}
}
func (e *Engine) Start() {
	go func() {
		defer close(e.done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-e.stop:
				return
			case <-ticker.C:
				e.Tick()
			}
		}
	}()
}

// Quiesce waits for submitted non-streaming calls. It never abandons a live host callback.
func (e *Engine) Close() {
	e.mu.Lock()
	if !e.stopped {
		e.stopped = true
		close(e.stop)
	}
	e.mu.Unlock()
	<-e.done
	e.workers.Wait()
}
func (e *Engine) Accounts() ([]Account, error) {
	var reply struct {
		Files []Account `json:"files"`
	}
	if err := e.host.Call("host.auth.list", struct{}{}, &reply); err != nil {
		return nil, errors.New("CPA account list unavailable")
	}
	result := []Account{}
	for _, a := range reply.Files {
		if a.Provider == "" {
			a.Provider = a.Type
		}
		if !providerOK(a.Provider) || a.ID == "" {
			continue
		}
		if a.Label != "" {
			a.Name = a.Label
		} else if a.Email != "" {
			a.Name = a.Email
		}
		if a.Name == "" {
			a.Name = a.ID
		}
		a.Plan = accountPlan(e.host, a)
		result = append(result, a)
	}
	return result, nil
}
func (e *Engine) Save(c Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped {
		return errors.New("plugin stopping")
	}
	if e.lastError != "" {
		return errors.New(e.lastError)
	}
	return e.store.Save(c)
}
func fingerprint(t Task) string {
	b := sha256.Sum256(marshal([]string{t.Provider, t.Model, t.Effort, "Reply with exactly: OK", "openai"}))
	return hex.EncodeToString(b[:])
}
func (e *Engine) enqueue(tasks []Task, c Config, at time.Time, mode, key string) ([]string, error) {
	tx, err := e.store.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	runIDs := []string{}
	for _, t := range tasks {
		claim := mode + ":" + t.ID + ":" + key
		var old string
		err = tx.QueryRow("SELECT run_id FROM claims WHERE key=?", claim).Scan(&old)
		if err == nil {
			runIDs = append(runIDs, old)
			continue
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
		rid := newID()
		if _, err = tx.Exec("INSERT INTO claims VALUES(?,?)", claim, rid); err != nil {
			return nil, err
		}
		if _, err = tx.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", rid, t.ID, t.Name, t.Model, t.Provider, mode, at.UnixMilli()); err != nil {
			return nil, err
		}
		runIDs = append(runIDs, rid)
		for _, id := range Targets(t, c.Groups) {
			dedup := mode + ":" + key + ":" + id + ":" + fingerprint(t)
			if mode == "manual" {
				dedup = claim + ":" + id
			}
			x := Execution{ID: newID(), AuthID: id, Provider: t.Provider, Model: t.Model, Status: "queued", Message: "Waiting for account lock / concurrency slot", Scheduled: at.UnixMilli(), Mode: mode, Task: t, Origins: []Origin{}}
			var prior string
			if err = tx.QueryRow("SELECT dedup FROM execution_keys WHERE dedup=?", dedup).Scan(&prior); err == nil {
				continue
			} else if err != sql.ErrNoRows {
				return nil, err
			}
			var raw []byte
			err = tx.QueryRow("SELECT body FROM executions WHERE dedup=?", dedup).Scan(&raw)
			if err == nil {
				if err = json.Unmarshal(raw, &x); err != nil {
					return nil, err
				}
			} else if err != sql.ErrNoRows {
				return nil, err
			}
			x.Origins = append(x.Origins, Origin{RunID: rid, TaskID: t.ID, Name: t.Name, Active: true})
			if _, err = tx.Exec("INSERT INTO executions VALUES(?,?,?) ON CONFLICT(dedup) DO UPDATE SET body=excluded.body", x.ID, dedup, marshal(x)); err != nil {
				return nil, err
			}
		}
	}
	return runIDs, tx.Commit()
}
func (e *Engine) Run(id, key string) (string, error) {
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
	for _, t := range c.Tasks {
		if t.ID == id {
			ids, err := e.enqueue([]Task{t}, c, e.now(), "manual", key)
			if err != nil {
				return "", err
			}
			return ids[0], nil
		}
	}
	return "", errors.New("task missing")
}
func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	xs, err := e.store.QueuedExecutions()
	if err != nil {
		return err
	}
	tx, err := e.store.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, x := range xs {
		if x.Status != "queued" {
			continue
		}
		valid := false
		for i := range x.Origins {
			if x.Origins[i].RunID == id {
				x.Origins[i].Active = false
			}
			valid = valid || x.Origins[i].Active
		}
		if !valid {
			x.Status = "skipped"
			x.Message = "Cancelled before dispatch"
			x.Finished = e.now().UnixMilli()
		}
		if _, err = tx.Exec("UPDATE executions SET body=? WHERE id=?", marshal(x), x.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (e *Engine) Tick() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.stopped || e.lastError != "" {
		return
	}
	c, err := e.store.Config()
	if err != nil {
		e.lastError = err.Error()
		return
	}
	now := e.now()
	if e.lastPruned.IsZero() || now.Sub(e.lastPruned) > time.Hour {
		if err = e.store.Prune(now, c.Settings.Retention); err != nil {
			e.lastError = err.Error()
			return
		}
		e.lastPruned = now
	}
	minute := now.Truncate(time.Minute)
	if minute.After(e.cursor) {
		from := e.cursor
		if minute.Sub(from) > time.Duration(c.Settings.Late)*time.Second {
			from = minute.Add(-time.Duration(c.Settings.Late) * time.Second)
		}
		if !c.Settings.Paused {
			for at := from.Add(time.Minute); !at.After(minute); at = at.Add(time.Minute) {
				if now.Sub(at) > time.Duration(c.Settings.Late)*time.Second {
					continue
				}
				due := []Task{}
				for _, t := range c.Tasks {
					if !t.Enabled {
						continue
					}
					if Matches(t, at) {
						due = append(due, t)
					}
				}
				if len(due) > 0 {
					if _, err = e.enqueue(due, c, at, "scheduled", at.UTC().Format(time.RFC3339)); err != nil {
						e.lastError = err.Error()
						return
					}
				}
			}
		}
		e.cursor = minute
	}
	xs, err := e.store.QueuedExecutions()
	if err != nil {
		e.lastError = err.Error()
		return
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
		} else if now.Sub(time.UnixMilli(x.Scheduled)) > time.Duration(c.Settings.Late)*time.Second {
			reason = "Queue deadline exceeded"
		}
		if reason != "" {
			x.Status = "skipped"
			x.Message = reason
			x.Finished = now.UnixMilli()
			if err = e.store.Put(x); err != nil {
				e.lastError = err.Error()
				return
			}
			continue
		}
		if len(e.active) >= c.Settings.Concurrency || e.active[x.AuthID] != "" || e.providerActive[x.Provider] >= 2 {
			continue
		}
		x.Status = "running"
		x.Started = now.UnixMilli()
		x.Message = "Request submitted; waiting for result"
		if err = e.store.Put(x); err != nil {
			e.lastError = err.Error()
			return
		}
		e.active[x.AuthID] = x.ID
		e.providerActive[x.Provider]++
		e.workers.Add(1)
		go e.execute(x)
	}
}
func (e *Engine) execute(x Execution) {
	defer e.workers.Done()
	defer func() {
		if recover() != nil {
			x.Status = "unknown"
			x.Message = "Host callback panicked; result unknown"
		}
		e.mu.Lock()
		defer e.mu.Unlock()
		x.Finished = e.now().UnixMilli()
		x.Latency = x.Finished - x.Started
		if err := e.store.Put(x); err != nil {
			e.lastError = "Cannot persist execution result; automatic dispatch stopped"
		}
		delete(e.active, x.AuthID)
		e.providerActive[x.Provider]--
	}()
	as, err := e.Accounts()
	if err != nil {
		x.Status = "skipped"
		x.Message = "Cannot verify target account; request not sent"
		return
	}
	var account *Account
	for i := range as {
		if as[i].ID == x.AuthID {
			account = &as[i]
			break
		}
	}
	if account == nil || account.Provider != x.Provider || account.Disabled || account.Unavailable || account.Status == "disabled" || account.Status == "error" || account.Status == "expired" {
		x.Status = "skipped"
		x.Message = "Account missing, unavailable, disabled or provider mismatch"
		return
	}
	// Recheck local pause after the read-only host lookup, then mark the dispatch boundary.
	e.mu.Lock()
	c, err := e.store.Config()
	stopped := e.stopped || err != nil || c.Settings.Paused || c.Overrides[x.AuthID]
	if !stopped && x.Mode == "scheduled" {
		valid := false
		for _, o := range x.Origins {
			for _, t := range c.Tasks {
				if o.Active && t.ID == o.TaskID && t.Enabled {
					valid = true
				}
			}
		}
		stopped = !valid
	}
	e.mu.Unlock()
	if stopped {
		x.Status = "skipped"
		x.Message = "Paused before model dispatch"
		return
	}
	request := buildModelRequest(x)
	var response ModelResponse
	err = e.host.Call("host.model.execute", request, &response)
	if err != nil {
		x.Status = "unknown"
		x.Message = "Host callback failed; result unknown. No retry."
		var he *HostError
		if errors.As(err, &he) && he.Status >= 400 {
			x.Status = "failed"
			x.HTTPStatus = he.Status
			x.Message = fmt.Sprintf("CPA error HTTP %d; no retry", he.Status)
		}
		return
	}
	x.HTTPStatus = response.StatusCode
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		x.Status = "failed"
		x.Message = fmt.Sprintf("HTTP %d; no automatic retry", response.StatusCode)
		return
	}
	var body struct {
		Choices []struct {
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens *int `json:"total_tokens"`
		} `json:"usage"`
		Error json.RawMessage `json:"error"`
	}
	valid := json.Unmarshal(response.Body, &body) == nil && len(body.Choices) > 0 && (len(body.Error) == 0 || string(body.Error) == "null")
	if valid {
		var content string
		if json.Unmarshal(body.Choices[0].Message.Content, &content) != nil || strings.TrimSpace(content) == "" {
			valid = false
		}
	}
	if !valid {
		x.Status = "unknown"
		x.Message = "Successful HTTP response without valid model content; no retry"
		return
	}
	x.Status = "success"
	x.Message = "Model replied successfully; actual quota reset remains unconfirmed"
	x.Tokens = body.Usage.TotalTokens
}

// CPA's official model suffix carries native thinking budgets and Gemini levels.
func buildModelRequest(x Execution) ModelRequest {
	model := x.Model
	effort := x.Task.Effort
	budget, numeric := strings.CutPrefix(effort, "budget:")
	if numeric {
		model += "(" + budget + ")"
	} else if x.Provider != "codex" && effort != "" {
		model += "(" + effort + ")"
	}
	payload := map[string]any{"model": model, "stream": false, "messages": []map[string]string{{"role": "user", "content": "Reply with exactly: OK"}}}
	if x.Provider == "codex" && effort != "" && !numeric {
		payload["reasoning_effort"] = effort
	}
	return ModelRequest{EntryProtocol: "openai", ExitProtocol: "openai", Model: model, AuthID: x.AuthID, ForcedProvider: x.Provider, Body: marshal(payload), Headers: httpHeaders(), Query: map[string][]string{}}
}
func httpHeaders() map[string][]string {
	return map[string][]string{"Content-Type": {"application/json"}}
}
func (e *Engine) Snapshot() (map[string]any, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, err := e.store.Config()
	if err != nil {
		return nil, err
	}
	// Summaries never carry batch items. Full details and account timelines have
	// separate on-demand endpoints, so history volume does not inflate polling.
	rows, err := e.store.db.Query(`WITH latest AS (SELECT (SELECT id FROM runs WHERE task_id=json_extract(t.value,'$.id') ORDER BY started DESC,id DESC LIMIT 1) id FROM json_each((SELECT body FROM config WHERE id=1),'$.tasks') t), selected_runs AS (SELECT * FROM runs WHERE id IN(SELECT id FROM latest) OR id IN(SELECT o.run_id FROM executions x JOIN history_origins o ON o.execution_id=x.id WHERE o.active AND json_extract(x.body,'$.status') IN ('queued','running'))), h AS (` + strings.Replace(historyTaskSQL, "FROM runs r LEFT JOIN", "FROM selected_runs r LEFT JOIN", 1) + `) SELECT id,taskId,name,model,provider,mode,started,status,accountCount FROM h ORDER BY started DESC,id DESC`)
	if err != nil {
		return nil, err
	}
	runs := []map[string]any{}
	for rows.Next() {
		var id, tid, name, model, provider, mode, status string
		var started int64
		var count int
		if err = rows.Scan(&id, &tid, &name, &model, &provider, &mode, &started, &status, &count); err != nil {
			rows.Close()
			return nil, err
		}
		runs = append(runs, map[string]any{"id": id, "taskId": tid, "name": name, "model": model, "provider": provider, "mode": mode, "started": started, "status": status, "finished": status != "running", "accountCount": count, "items": []map[string]any{}, "summary": true})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	zone, err := time.LoadLocation(c.Settings.Timezone)
	if err != nil {
		return nil, err
	}
	local := e.now().In(zone)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, zone)
	var attempted, success int
	err = e.store.db.QueryRow(`SELECT coalesce(sum(json_extract(x.body,'$.status') IN ('success','failed')),0),coalesce(sum(json_extract(x.body,'$.status')='success'),0) FROM executions x WHERE x.id IN(SELECT o.execution_id FROM runs r JOIN history_origins o ON o.run_id=r.id WHERE r.started>=? AND r.started<?)`, start.UnixMilli(), start.AddDate(0, 0, 1).UnixMilli()).Scan(&attempted, &success)
	if err != nil {
		return nil, err
	}
	return map[string]any{"config": c, "runs": runs, "todayStats": map[string]int{"attempted": attempted, "success": success}, "now": e.now().UnixMilli(), "lastError": e.lastError, "version": Version, "active": len(e.active)}, nil
}

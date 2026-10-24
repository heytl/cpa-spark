package warmup

import (
	"errors"
)

type HistoryFilter struct {
	AccountMatches []string `json:"accountMatches"`
	TaskID         string   `json:"taskId"`
	Query          string   `json:"query"`
	Date           string   `json:"date"`
	Status         string   `json:"status"`
	Timezone       string   `json:"timezone"`
}
type HistoryDelete struct {
	Mode         string         `json:"mode"`
	ExecutionIDs []string       `json:"executionIds"`
	IDs          []string       `json:"ids"`
	All          bool           `json:"all"`
	Filter       *HistoryFilter `json:"filter"`
	Preview      bool           `json:"preview"`
}

// DeleteHistory preserves claims and dedup keys, and never deletes active records.
func (e *Engine) DeleteHistory(req HistoryDelete) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.deleteHistorySQL(req)
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

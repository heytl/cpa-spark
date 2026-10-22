package warmup

import (
	"fmt"
	"testing"
	"time"
)

func TestHistoryDeletionScopesKeepActiveAndClaims(t *testing.T) {
	e, s := engineFixture(t, &fakeHost{})
	at := time.Date(2026, 10, 21, 0, 0, 5, 0, time.UTC).UnixMilli()
	for i := 0; i < 205; i++ {
		id := fmt.Sprintf("run-%03d", i)
		if _, err := s.db.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", id, "t", "Warmup", "model", "codex", "manual", at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", "active", "t", "Active", "model", "codex", "manual", at); err != nil {
		t.Fatal(err)
	}
	x := Execution{ID: "active-x", Status: "running", Origins: []Origin{{RunID: "active", Active: true}}}
	if _, err := s.db.Exec("INSERT INTO executions VALUES(?,?,?)", x.ID, "dedup-active", marshal(x)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("INSERT INTO claims VALUES(?,?)", "manual:t:test-key", "run-000"); err != nil {
		t.Fatal(err)
	}
	preview, err := e.DeleteHistory(HistoryDelete{All: true, Preview: true})
	if err != nil || preview != 205 {
		t.Fatalf("preview all beyond 200: %d %v", preview, err)
	}
	n, err := e.DeleteHistory(HistoryDelete{Filter: &HistoryFilter{Date: "2026-10-20", Timezone: "America/Los_Angeles", Query: "run-001", Status: "skipped"}})
	if err != nil || n != 1 {
		t.Fatalf("filtered timezone: %d %v", n, err)
	}
	n, err = e.DeleteHistory(HistoryDelete{IDs: []string{"active", "run-000"}})
	if err != nil || n != 1 {
		t.Fatalf("single preserves running: %d %v", n, err)
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM claims").Scan(&count)
	if count != 1 {
		t.Fatal("deletion removed idempotency claim")
	}
	n, err = e.DeleteHistory(HistoryDelete{All: true})
	if err != nil || n != 203 {
		t.Fatalf("all: %d %v", n, err)
	}
	s.db.QueryRow("SELECT count(*) FROM runs").Scan(&count)
	if count != 1 {
		t.Fatal("active run deleted")
	}
	if _, err = e.DeleteHistory(HistoryDelete{}); err == nil {
		t.Fatal("missing scope accepted")
	}
	if _, err = e.DeleteHistory(HistoryDelete{All: true, IDs: []string{"active"}}); err == nil {
		t.Fatal("ambiguous scope accepted")
	}
}

func TestRunOnceDoesNotSaveTaskAndIsIdempotent(t *testing.T) {
	e, s := engineFixture(t, &fakeHost{accounts: []Account{{ID: "a", Provider: "codex"}}})
	task := taskFixture("temporary")
	id, err := e.RunOnce(task, "one-off-test-key")
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.RunOnce(task, "one-off-test-key")
	if err != nil || again != id {
		t.Fatalf("duplicate one-off: %s %v", again, err)
	}
	cfg, err := s.Config()
	if err != nil || len(cfg.Tasks) != 0 {
		t.Fatal("one-off task persisted in scheduled config")
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM runs").Scan(&count)
	if count != 1 {
		t.Fatalf("one-off runs: %d", count)
	}
	task.Model = ""
	if _, err = e.RunOnce(task, "invalid-once-key"); err == nil {
		t.Fatal("invalid model accepted")
	}
}

func TestSnapshotRetainsAllRecentRunsAndActualExecutionTimes(t *testing.T) {
	e, s := engineFixture(t, goodHost())
	now := e.now().UnixMilli()
	for i := 0; i < 205; i++ {
		if _, err := s.db.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", fmt.Sprint("recent-", i), "t", "Recent", "m", "codex", "manual", now-1000); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", "old-start", "t", "Delayed", "m", "codex", "manual", now-48*60*60*1000); err != nil {
		t.Fatal(err)
	}
	x := Execution{ID: "delayed-result", AuthID: "a", Status: "success", Started: now - 2000, Finished: now - 1000, Origins: []Origin{{RunID: "old-start", TaskID: "t", Name: "Delayed", Active: true}}}
	if _, err := s.db.Exec("INSERT INTO executions VALUES(?,?,?)", x.ID, "delayed-key", marshal(x)); err != nil {
		t.Fatal(err)
	}

	snap, err := e.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	runs := snap["runs"].([]map[string]any)
	if len(runs) != 206 {
		t.Fatalf("recent runs truncated: %d", len(runs))
	}
	found := false
	for _, r := range runs {
		if r["id"] == "old-start" {
			items := r["items"].([]map[string]any)
			if len(items) != 1 || items[0]["finished"] != now-1000 {
				t.Fatalf("missing actual execution time: %+v", items)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("delayed execution missing from recent history")
	}
}

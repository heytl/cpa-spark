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

func TestTimelineRetainsActualExecutionTimesWithoutInflatingSnapshot(t *testing.T) {
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
	if len(snap["runs"].([]map[string]any)) != 0 {
		t.Fatal("removed task history leaked into home polling")
	}
	page, err := e.History(HistoryQuery{Mode: "task", Page: 1, PageSize: 50})
	if err != nil || page["total"] != 206 || len(page["rows"].([]map[string]any)) != 50 {
		t.Fatal(page, err)
	}
	timeline, err := e.AccountTimeline("a")
	if err != nil {
		t.Fatal(err)
	}
	runs := timeline["runs"].([]map[string]any)
	if len(runs) != 1 {
		t.Fatalf("account timeline contains unrelated runs: %d", len(runs))
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

func TestAccountHistoryDeletionRetainsOtherAccountsAndDedupKeys(t *testing.T) {
	e, s := engineFixture(t, &fakeHost{})
	for _, id := range []string{"r1", "r2"} {
		if _, err := s.db.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", id, "t", "Task", "m", "codex", "scheduled", e.now().UnixMilli()); err != nil {
			t.Fatal(err)
		}
	}
	xs := []Execution{
		{ID: "x1", AuthID: "a", Status: "success", Origins: []Origin{{RunID: "r1", Active: true}, {RunID: "r2", Active: true}}},
		{ID: "x2", AuthID: "b", Status: "success", Origins: []Origin{{RunID: "r1", Active: true}}},
		{ID: "x3", AuthID: "c", Status: "running", Origins: []Origin{{RunID: "r2", Active: true}}},
	}
	for _, x := range xs {
		if _, err := s.db.Exec("INSERT INTO executions VALUES(?,?,?)", x.ID, "dedup-"+x.ID, marshal(x)); err != nil {
			t.Fatal(err)
		}
	}
	req := HistoryDelete{Mode: "account", ExecutionIDs: []string{"x1", "x1", "x3"}, Preview: true}
	if n, err := e.DeleteHistory(req); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	req.Preview = false
	if n, err := e.DeleteHistory(req); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	remaining, err := s.Executions()
	if err != nil || len(remaining) != 2 {
		t.Fatal(remaining, err)
	}
	var key string
	if err := s.db.QueryRow("SELECT dedup FROM execution_keys WHERE dedup=?", "dedup-x1").Scan(&key); err != nil {
		t.Fatal("charged-request key lost", err)
	}
	if n, err := e.DeleteHistory(HistoryDelete{Mode: "account", All: true}); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	var count int
	s.db.QueryRow("SELECT count(*) FROM runs").Scan(&count)
	if count != 1 {
		t.Fatal("active run removed", count)
	}
	if _, err := e.DeleteHistory(HistoryDelete{ExecutionIDs: []string{"x3"}}); err == nil {
		t.Fatal("execution IDs accepted without account mode")
	}
}

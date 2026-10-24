package warmup

import (
	"fmt"
	"testing"
	"time"
)

func TestHistoryPagesFiltersAndDeletionAcrossAllPages(t *testing.T) {
	e, s := engineFixture(t, goodHost())
	now := e.now().UnixMilli()
	for i := 0; i < 237; i++ {
		id := fmt.Sprintf("r-%03d", i)
		if _, err := s.db.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", id, "t", "Task "+id, "m", "codex", "scheduled", now-int64(i)*1000); err != nil {
			t.Fatal(err)
		}
		status := "success"
		if i%10 == 0 {
			status = "failed"
		}
		if i == 236 {
			status = "running"
		}
		x := Execution{ID: "x-" + id, AuthID: "a", Model: "m", Status: status, Started: now - int64(i)*1000, Finished: now, Origins: []Origin{{RunID: id, TaskID: "t", Name: "Task " + id, Active: true}}}
		if _, err := s.db.Exec("INSERT INTO executions VALUES(?,?,?)", x.ID, x.ID, marshal(x)); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for page := 1; page <= 12; page++ {
		v, err := e.History(HistoryQuery{Mode: "account", Page: page, PageSize: 20})
		if err != nil {
			t.Fatal(err)
		}
		if v["total"] != 237 || v["pages"] != 12 || v["deletable"] != 236 {
			t.Fatal(v)
		}
		rows := v["rows"].([]map[string]any)
		if len(rows) > 20 {
			t.Fatal("unbounded page")
		}
		for _, row := range rows {
			id := row["id"].(string)
			if seen[id] {
				t.Fatal("unstable pagination", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 237 {
		t.Fatal("missing records", len(seen))
	}
	// A shared request remains one account record but appears in both task batches.
	if _, err := s.db.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", "shared", "other", "Shared", "m", "codex", "scheduled", now); err != nil {
		t.Fatal(err)
	}
	xs, _ := s.Executions()
	shared := xs[0]
	shared.Origins = append(shared.Origins, Origin{RunID: "shared", TaskID: "other", Name: "Shared", Active: true})
	if err := s.Put(shared); err != nil {
		t.Fatal(err)
	}
	v, err := e.History(HistoryQuery{Mode: "task", PageSize: 50})
	if err != nil || v["total"] != 238 {
		t.Fatal(v, err)
	}
	v, err = e.History(HistoryQuery{Mode: "account", Filter: HistoryFilter{TaskID: "other"}, PageSize: 50})
	if err != nil || v["total"] != 1 {
		t.Fatal(v, err)
	}
	v, err = e.History(HistoryQuery{Mode: "account", Filter: HistoryFilter{Query: "Saved account label", AccountMatches: []string{"a"}}, PageSize: 50})
	if err != nil || v["total"] != 237 {
		t.Fatal(v, err)
	}
	filter := HistoryFilter{Status: "success", Timezone: "Asia/Shanghai"}
	preview := HistoryDelete{Mode: "account", Filter: &filter, Preview: true}
	n, err := e.DeleteHistory(preview)
	if err != nil || n != 212 {
		t.Fatal(n, err)
	}
	preview.Preview = false
	n, err = e.DeleteHistory(preview)
	if err != nil || n != 212 {
		t.Fatal("cross-page deletion", n, err)
	}
	v, err = e.History(HistoryQuery{Mode: "account", Page: 500, PageSize: 20})
	if err != nil || v["total"] != 25 || v["page"] != 2 {
		t.Fatal(v, err)
	}
	if _, err = e.History(HistoryQuery{Mode: "account", PageSize: 1000}); err == nil {
		t.Fatal("unbounded limit accepted")
	}
	if _, err = e.History(HistoryQuery{Mode: "account", Filter: HistoryFilter{Date: "2026-02-30"}}); err == nil {
		t.Fatal("invalid date accepted")
	}
	if _, err = e.HistoryDetail("r-236"); err != nil {
		t.Fatal("running detail lost", err)
	}
}

func TestHistoryDateUsesLocalDayAndDetailOutsideSnapshot(t *testing.T) {
	e, s := engineFixture(t, goodHost())
	at := time.Date(2026, 10, 23, 16, 0, 0, 0, time.UTC).UnixMilli()
	_, err := s.db.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", "old", "removed", "Historical", "m", "codex", "manual", at)
	if err != nil {
		t.Fatal(err)
	}
	x := Execution{ID: "old-x", AuthID: "a", Model: "m", Started: at, Finished: at + 1, Status: "failed", Origins: []Origin{{RunID: "old", TaskID: "removed", Active: true}}}
	if _, err = s.db.Exec("INSERT INTO executions VALUES(?,?,?)", x.ID, x.ID, marshal(x)); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"account", "task"} {
		v, err := e.History(HistoryQuery{Mode: mode, Filter: HistoryFilter{Date: "2026-10-24", Timezone: "Asia/Shanghai"}})
		if err != nil || v["total"] != 1 {
			t.Fatal(v, err)
		}
		v, err = e.History(HistoryQuery{Mode: mode, Filter: HistoryFilter{Date: "2026-10-23", Timezone: "Asia/Shanghai"}})
		if err != nil || v["total"] != 0 {
			t.Fatal(v, err)
		}
	}
	snapshot, err := e.Snapshot()
	if err != nil || len(snapshot["runs"].([]map[string]any)) != 0 {
		t.Fatal(snapshot, err)
	}
	detail, err := e.HistoryDetail("old")
	if err != nil || len(detail["items"].([]map[string]any)) != 1 {
		t.Fatal(detail, err)
	}
}

func TestHistoryIndexBackfillsExistingExecutionsAndTracksMutations(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	x := Execution{ID: "x", Status: "success", Origins: []Origin{{RunID: "one", Active: true}}}
	if _, err = s.db.Exec("INSERT INTO executions VALUES(?,?,?)", x.ID, x.ID, marshal(x)); err != nil {
		t.Fatal(err)
	}
	// Simulate the pre-index schema used by existing installations.
	if _, err = s.db.Exec("DROP TRIGGER history_execution_insert;DROP TRIGGER history_execution_update;DROP TRIGGER history_execution_delete;DROP TABLE history_origins"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal("upgrade", err)
	}
	defer s.Close()
	var id string
	if err = s.db.QueryRow("SELECT run_id FROM history_origins WHERE execution_id='x'").Scan(&id); err != nil || id != "one" {
		t.Fatal(id, err)
	}
	x.Origins = []Origin{{RunID: "two", Active: false}}
	if err = s.Put(x); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = s.db.QueryRow("SELECT count(*) FROM history_origins WHERE run_id='one'").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if _, err = s.db.Exec("DELETE FROM executions WHERE id='x'"); err != nil {
		t.Fatal(err)
	}
	if err = s.db.QueryRow("SELECT count(*) FROM history_origins").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func BenchmarkHistoryPages(b *testing.B) {
	s, err := Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	c, _ := s.Config()
	c.Tasks = []Task{taskFixture("bench")}
	if err = s.Save(c); err != nil {
		b.Fatal(err)
	}
	now := time.Now().UnixMilli()
	tx, err := s.db.Begin()
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 10000; i++ {
		id := fmt.Sprintf("bench-%05d", i)
		if _, err = tx.Exec("INSERT INTO runs VALUES(?,?,?,?,?,?,?)", id, "bench", "Benchmark", "m", "codex", "scheduled", now-int64(i)*1000); err != nil {
			b.Fatal(err)
		}
		x := Execution{ID: id, AuthID: "a", Model: "m", Status: "success", Started: now - int64(i)*1000, Finished: now, Origins: []Origin{{RunID: id, TaskID: "bench", Name: "Benchmark", Active: true}}}
		if _, err = tx.Exec("INSERT INTO executions VALUES(?,?,?)", id, id, marshal(x)); err != nil {
			b.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		b.Fatal(err)
	}
	e := NewEngine(s, &fakeHost{}, time.Now)
	for _, mode := range []string{"account", "task"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				page, err := e.History(HistoryQuery{Mode: mode, Page: 2, PageSize: 50})
				if err != nil {
					b.Fatal(err)
				}
				if len(page["rows"].([]map[string]any)) != 50 {
					b.Fatal("page not bounded")
				}
			}
		})
	}
	b.Run("home", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			snapshot, err := e.Snapshot()
			if err != nil {
				b.Fatal(err)
			}
			if len(snapshot["runs"].([]map[string]any)) != 1 {
				b.Fatal("summary not bounded")
			}
		}
	})
}

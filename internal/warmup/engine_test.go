package warmup

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeHost struct {
	mu       sync.Mutex
	accounts []Account
	calls    []ModelRequest
	gate     chan struct{}
	response ModelResponse
	err      error
}

func (f *fakeHost) Call(method string, in any, out any) error {
	f.mu.Lock()
	if method == "host.auth.list" {
		b := marshal(map[string]any{"files": f.accounts})
		f.mu.Unlock()
		return json.Unmarshal(b, out)
	}
	req := in.(ModelRequest)
	f.calls = append(f.calls, req)
	gate := f.gate
	response := f.response
	err := f.err
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	if err != nil {
		return err
	}
	b := marshal(response)
	return json.Unmarshal(b, out)
}
func taskFixture(id string) Task {
	return Task{ID: id, Name: id, Provider: "codex", AccountIDs: []string{"a"}, GroupIDs: []string{}, Excludes: []string{}, Days: []int{0, 1, 2, 3, 4, 5, 6}, Times: []string{"06:00"}, Timezone: "Asia/Shanghai", Model: "model-test", Effort: "low", Enabled: true, Revision: 1}
}
func engineFixture(t *testing.T, host *fakeHost) (*Engine, *Store) {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 20, 21, 59, 59, 0, time.UTC)
	e := NewEngine(s, host, func() time.Time { return now })
	e.Start()
	t.Cleanup(func() { e.Close(); s.Close() })
	return e, s
}
func goodHost() *fakeHost {
	return &fakeHost{accounts: []Account{{ID: "a", Provider: "codex", Status: "active"}, {ID: "b", Provider: "codex", Status: "active"}, {ID: "c", Provider: "antigravity", Status: "active"}}, response: ModelResponse{StatusCode: 200, Body: []byte(`{"choices":[{"message":{"content":"OK"}}],"usage":{"total_tokens":8}}`)}}
}
func mustSave(t *testing.T, s *Store, tasks ...Task) Config {
	t.Helper()
	c, _ := s.Config()
	c.Tasks = tasks
	if err := s.Save(c); err != nil {
		t.Fatal(err)
	}
	c, _ = s.Config()
	return c
}
func waitFor(t *testing.T, check func() bool) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		if check() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition timed out")
}
func TestScheduleWeeklyAndDST(t *testing.T) {
	task := taskFixture("weekly")
	task.Days = []int{6}
	task.Times = []string{"09:00"}
	after := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	got := Next(task, after, 5)
	if len(got) != 5 || got[0].In(mustLocation("Asia/Shanghai")).Format("2006-01-02") != "2026-10-24" {
		t.Fatalf("weekly preview: %v", got)
	}
	task.Timezone = "America/New_York"
	task.Days = []int{0}
	task.Times = []string{"02:30"}
	got = Next(task, time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC), 1)
	if got[0].In(mustLocation(task.Timezone)).Day() != 15 {
		t.Fatalf("gap not skipped: %v", got)
	}
	task.Times = []string{"01:30"}
	got = Next(task, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), 2)
	if !got[0].Equal(time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)) || got[1].In(mustLocation(task.Timezone)).Day() != 8 {
		t.Fatalf("fold duplicated: %v", got)
	}
}
func mustLocation(s string) *time.Location {
	l, e := time.LoadLocation(s)
	if e != nil {
		panic(e)
	}
	return l
}
func TestScheduledMergeAndExactBinding(t *testing.T) {
	h := goodHost()
	e, s := engineFixture(t, h)
	a, b := taskFixture("one"), taskFixture("two")
	c := mustSave(t, s, a, b)
	at := e.now()
	e.mu.Lock()
	ids, err := e.enqueue(c.Tasks, c, at, "scheduled", at.Format(time.RFC3339))
	e.mu.Unlock()
	if err != nil || len(ids) != 2 {
		t.Fatal(ids, err)
	}
	e.Tick()
	waitFor(t, func() bool { xs, _ := s.Executions(); return len(xs) == 1 && xs[0].Status == "success" })
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.calls) != 1 || h.calls[0].AuthID != "a" || h.calls[0].ForcedProvider != "codex" {
		t.Fatal(h.calls)
	}
	xs, _ := s.Executions()
	if len(xs[0].Origins) != 2 {
		t.Fatal(xs)
	}
	snapshot, err := e.Snapshot()
	if err != nil || len(snapshot["runs"].([]map[string]any)) != 2 {
		t.Fatal(snapshot, err)
	}
}
func TestManualIdempotencyAndAccountSerialization(t *testing.T) {
	h := goodHost()
	h.gate = make(chan struct{}, 4)
	e, s := engineFixture(t, h)
	a, b := taskFixture("one"), taskFixture("two")
	b.Model = "different"
	mustSave(t, s, a, b)
	id, err := e.Run("one", "click-key-123")
	if err != nil {
		t.Fatal(err)
	}
	again, err := e.Run("one", "click-key-123")
	if err != nil || id != again {
		t.Fatal(again, err)
	}
	e.Run("two", "click-key-456")
	e.Tick()
	waitFor(t, func() bool { h.mu.Lock(); defer h.mu.Unlock(); return len(h.calls) == 1 })
	e.Tick()
	h.mu.Lock()
	if len(h.calls) != 1 {
		t.Fatal("overlapping account requests")
	}
	h.mu.Unlock()
	h.gate <- struct{}{}
	waitFor(t, func() bool { e.mu.Lock(); defer e.mu.Unlock(); return len(e.active) == 0 })
	e.Tick()
	waitFor(t, func() bool { h.mu.Lock(); defer h.mu.Unlock(); return len(h.calls) == 2 })
	h.gate <- struct{}{}
	waitFor(t, func() bool { e.mu.Lock(); defer e.mu.Unlock(); return len(e.active) == 0 })
}
func TestPauseSkipsQueueButKeepsInflight(t *testing.T) {
	h := goodHost()
	h.gate = make(chan struct{}, 4)
	e, s := engineFixture(t, h)
	c := mustSave(t, s, taskFixture("one"))
	e.Run("one", "first-click")
	e.Run("one", "second-click")
	e.Tick()
	waitFor(t, func() bool { h.mu.Lock(); defer h.mu.Unlock(); return len(h.calls) == 1 })
	c.Settings.Paused = true
	if err := e.Save(c); err != nil {
		t.Fatal(err)
	}
	xs, _ := s.Executions()
	if xs[0].Status != "running" || xs[1].Status != "skipped" {
		t.Fatal(xs)
	}
	h.gate <- struct{}{}
	waitFor(t, func() bool { xs, _ := s.Executions(); return xs[0].Status == "success" })
}
func TestCancelOneMergedOrigin(t *testing.T) {
	e, s := engineFixture(t, goodHost())
	c := mustSave(t, s, taskFixture("one"), taskFixture("two"))
	ids, err := e.enqueue(c.Tasks, c, e.now(), "scheduled", "at")
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Cancel(ids[0]); err != nil {
		t.Fatal(err)
	}
	xs, _ := s.Executions()
	if xs[0].Status != "queued" || xs[0].Origins[0].Active || !xs[0].Origins[1].Active {
		t.Fatal(xs)
	}
	e.Tick()
	waitFor(t, func() bool { xs, _ := s.Executions(); return xs[0].Status == "success" })
}
func TestMissingDisabledNoFallback(t *testing.T) {
	h := goodHost()
	h.accounts[0].Disabled = true
	e, s := engineFixture(t, h)
	mustSave(t, s, taskFixture("one"))
	e.Run("one", "disabled-click")
	e.Tick()
	waitFor(t, func() bool { xs, _ := s.Executions(); return xs[0].Status == "skipped" })
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.calls) != 0 {
		t.Fatal("disabled account invoked")
	}
}
func TestUnknownResultNotRetried(t *testing.T) {
	h := goodHost()
	h.err = errors.New("transport lost")
	e, s := engineFixture(t, h)
	mustSave(t, s, taskFixture("one"))
	e.Run("one", "lost-click")
	e.Tick()
	waitFor(t, func() bool { xs, _ := s.Executions(); return xs[0].Status == "unknown" })
	e.Tick()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.calls) != 1 {
		t.Fatal("unknown result retried")
	}
}
func TestStoreReopenLockAndConflict(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if other, err := Open(dir); err == nil {
		other.Close()
		t.Fatal("second process lock succeeded")
	}
	c := mustSave(t, s, taskFixture("one"))
	if err = s.Save(Config{Revision: 0, Settings: c.Settings}); err != ErrConflict {
		t.Fatal(err)
	}
	x := Execution{ID: "crashed", Status: "running", AuthID: "a", Origins: []Origin{}}
	s.db.Exec("INSERT INTO executions VALUES(?,?,?)", x.ID, "key", marshal(x))
	s.Close()
	s, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	xs, _ := s.Executions()
	if xs[0].Status != "unknown" {
		t.Fatal(xs)
	}
	got, _ := s.Config()
	if len(got.Tasks) != 1 {
		t.Fatal(got)
	}
}
func TestProviderAndGlobalLimits(t *testing.T) {
	h := goodHost()
	h.gate = make(chan struct{}, 10)
	h.accounts = append(h.accounts, Account{ID: "d", Provider: "codex", Status: "active"})
	e, s := engineFixture(t, h)
	a := taskFixture("codex")
	a.AccountIDs = []string{"a", "b", "d"}
	b := taskFixture("ag")
	b.Provider = "antigravity"
	b.AccountIDs = []string{"c"}
	c := mustSave(t, s, a, b)
	c.Settings.Concurrency = 3
	if err := e.Save(c); err != nil {
		t.Fatal(err)
	}
	e.Run("codex", "limit-codex")
	e.Run("ag", "limit-antigravity")
	e.Tick()
	waitFor(t, func() bool { h.mu.Lock(); defer h.mu.Unlock(); return len(h.calls) == 3 })
	e.mu.Lock()
	if len(e.active) != 3 || e.providerActive["codex"] != 2 {
		t.Error(e.active, e.providerActive)
	}
	e.mu.Unlock()
	for i := 0; i < 3; i++ {
		h.gate <- struct{}{}
	}
	waitFor(t, func() bool { e.mu.Lock(); defer e.mu.Unlock(); return len(e.active) == 0 })
	e.Tick()
	waitFor(t, func() bool { h.mu.Lock(); defer h.mu.Unlock(); return len(h.calls) == 4 })
	h.gate <- struct{}{}
	waitFor(t, func() bool { e.mu.Lock(); defer e.mu.Unlock(); return len(e.active) == 0 })
}

func TestExpiredAccountIsSkippedWithoutModelRequest(t *testing.T) {
	h := goodHost()
	h.accounts[0].Status = "expired"
	e, s := engineFixture(t, h)
	mustSave(t, s, taskFixture("expired"))
	e.Run("expired", "expired-key")
	e.Tick()
	waitFor(t, func() bool { xs, _ := s.Executions(); return len(xs) > 0 && xs[0].Status == "skipped" })
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.calls) != 0 {
		t.Fatal("expired account invoked")
	}
}

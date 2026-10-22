package warmup

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"
)

const PluginID = "cpa-spark"
const ProjectName = "CPA Spark"

// CPA requires author and repository metadata. Fork release builds may override them.
var Author = "heytl"
var Repository = "https://github.com/heytl/cpa-spark"

const ProjectDescription = "为 CLIProxyAPI 提供轻量的账号定时预热、分组调度与额度窗口可视化。"
const Version = "0.1.7"

type Host interface{ Call(string, any, any) error }
type HostError struct {
	Code   string
	Status int
}

func (e *HostError) Error() string { return "CPA callback failed: " + e.Code }

// Host account fields follow CPA sdk/pluginapi.HostAuthFileEntry.
// Optional subscription fields are read only when explicitly supplied by CPA.
type Account struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Provider        string `json:"provider"`
	Type            string `json:"type,omitempty"`
	Label           string `json:"label,omitempty"`
	Email           string `json:"email,omitempty"`
	Status          string `json:"status,omitempty"`
	Disabled        bool   `json:"disabled"`
	Unavailable     bool   `json:"unavailable"`
	AccountType     string `json:"account_type,omitempty"`
	PlanType        string `json:"plan_type,omitempty"`
	Tier            string `json:"tier,omitempty"`
	ChatGPTPlanType string `json:"chatgpt_plan_type,omitempty"`
	AuthIndex       string `json:"auth_index,omitempty"`
	Plan            string `json:"plan,omitempty"`
}
type Group struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Provider    string   `json:"provider"`
	Members     []string `json:"members"`
	Color       string   `json:"color"`
	Description string   `json:"description"`
}
type Task struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Provider   string   `json:"provider"`
	GroupIDs   []string `json:"groupIds"`
	AccountIDs []string `json:"accountIds"`
	Excludes   []string `json:"excludes"`
	Days       []int    `json:"days"`
	Times      []string `json:"times"`
	Timezone   string   `json:"timezone"`
	Model      string   `json:"model"`
	Effort     string   `json:"effort"`
	Enabled    bool     `json:"enabled"`
	Revision   int      `json:"revision"`
}
type Settings struct {
	Timezone    string `json:"timezone"`
	Concurrency int    `json:"concurrency"`
	Late        int    `json:"late"`
	Retention   int    `json:"retention"`
	Paused      bool   `json:"paused"`
}
type Config struct {
	Revision  int             `json:"revision"`
	Tasks     []Task          `json:"tasks"`
	Groups    []Group         `json:"groups"`
	Settings  Settings        `json:"settings"`
	Overrides map[string]bool `json:"overrides"`
}

func DefaultConfig() Config {
	return Config{Tasks: []Task{}, Groups: []Group{}, Overrides: map[string]bool{}, Settings: Settings{Timezone: "Asia/Shanghai", Concurrency: 4, Late: 120, Retention: 30}}
}
func newID() string {
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func providerOK(p string) bool { return p == "codex" || p == "antigravity" }

var hhmm = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)

func Validate(c *Config) error {
	if c.Settings.Concurrency < 1 || c.Settings.Concurrency > 8 || c.Settings.Late < 60 || c.Settings.Late > 300 || c.Settings.Retention < 1 || c.Settings.Retention > 365 {
		return errors.New("invalid scheduler limits")
	}
	if _, e := time.LoadLocation(c.Settings.Timezone); e != nil {
		return e
	}
	if len(c.Tasks) > 500 || len(c.Groups) > 500 || len(c.Overrides) > 5000 {
		return errors.New("configuration limit exceeded")
	}
	gs := map[string]Group{}
	ids := map[string]bool{}
	for _, g := range c.Groups {
		if g.ID == "" || ids[g.ID] || len(g.ID) > 128 || strings.TrimSpace(g.Name) == "" || len(g.Name) > 200 || !providerOK(g.Provider) || len(g.Members) > 5000 {
			return errors.New("invalid group")
		}
		ids[g.ID] = true
		gs[g.ID] = g
	}
	ids = map[string]bool{}
	for _, t := range c.Tasks {
		if t.ID == "" || ids[t.ID] || len(t.ID) > 128 || strings.TrimSpace(t.Name) == "" || len(t.Name) > 200 || !providerOK(t.Provider) || strings.TrimSpace(t.Model) == "" || len(t.Model) > 200 || len(t.Times) == 0 || len(t.Times) > 24 || len(t.Days) == 0 || len(t.AccountIDs) > 5000 || len(t.GroupIDs) > 500 || len(t.Excludes) > 5000 {
			return errors.New("invalid task")
		}
		ids[t.ID] = true
		if _, e := time.LoadLocation(t.Timezone); e != nil {
			return e
		}
		for _, d := range t.Days {
			if d < 0 || d > 6 {
				return errors.New("invalid weekday")
			}
		}
		for _, v := range t.Times {
			if !hhmm.MatchString(v) {
				return errors.New("invalid HH:MM")
			}
		}
		if !validEffort(t.Effort) {
			return errors.New("unsupported reasoning effort")
		}
		for _, id := range t.GroupIDs {
			g, ok := gs[id]
			if !ok || g.Provider != t.Provider {
				return errors.New("missing or incompatible group")
			}
		}
		if t.Enabled && len(Targets(t, c.Groups)) == 0 {
			return fmt.Errorf("task %s has no targets", t.Name)
		}
	}
	if c.Overrides == nil {
		c.Overrides = map[string]bool{}
	}
	return nil
}

func validEffort(effort string) bool {
	switch effort {
	case "", "none", "minimal", "low", "medium", "high", "xhigh", "max", "auto":
		return true
	}
	if raw, ok := strings.CutPrefix(effort, "budget:"); ok {
		budget, err := strconv.Atoi(raw)
		return err == nil && budget >= 0 && budget <= 1000000 && strconv.Itoa(budget) == raw
	}
	return false
}
func Targets(t Task, groups []Group) []string {
	set := map[string]bool{}
	for _, id := range t.AccountIDs {
		set[id] = true
	}
	for _, g := range groups {
		for _, id := range t.GroupIDs {
			if g.ID == id {
				for _, a := range g.Members {
					set[a] = true
				}
			}
		}
	}
	for _, id := range t.Excludes {
		delete(set, id)
	}
	delete(set, "")
	a := []string{}
	for id := range set {
		a = append(a, id)
	}
	sort.Strings(a)
	return a
}

// Enumerating UTC minutes naturally skips DST gaps. The first wall-clock occurrence wins in folds.
func Next(t Task, after time.Time, count int) []time.Time {
	result := []time.Time{}
	if count < 1 || count > 100 {
		return result
	}
	loc, e := time.LoadLocation(t.Timezone)
	if e != nil {
		return result
	}
	days := map[int]bool{}
	times := map[string]bool{}
	for _, d := range t.Days {
		days[d] = true
	}
	for _, v := range t.Times {
		times[v] = true
	}
	end := after.AddDate(0, 0, 7*count+9)
	for at := after.Truncate(time.Minute).Add(time.Minute); at.Before(end) && len(result) < count; at = at.Add(time.Minute) {
		l := at.In(loc)
		if !days[int(l.Weekday())] || !times[l.Format("15:04")] {
			continue
		}
		duplicate := false
		for back := at.Add(-3 * time.Hour); back.Before(at); back = back.Add(time.Minute) {
			if back.In(loc).Format("2006-01-02 15:04") == l.Format("2006-01-02 15:04") {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, at)
		}
	}
	return result
}

type ModelRequest struct {
	EntryProtocol  string              `json:"entry_protocol"`
	ExitProtocol   string              `json:"exit_protocol"`
	Model          string              `json:"model"`
	Stream         bool                `json:"stream"`
	Body           []byte              `json:"body"`
	Headers        http.Header         `json:"headers"`
	Query          map[string][]string `json:"query"`
	ForcedProvider string              `json:"forced_provider"`
	AuthID         string              `json:"auth_id"`
}
type ModelResponse struct {
	StatusCode int         `json:"status_code"`
	Body       []byte      `json:"body"`
	Headers    http.Header `json:"headers"`
}
type Origin struct {
	RunID  string `json:"runId"`
	TaskID string `json:"taskId"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}
type Execution struct {
	ID         string   `json:"id"`
	AuthID     string   `json:"authId"`
	Provider   string   `json:"provider"`
	Model      string   `json:"model"`
	Status     string   `json:"status"`
	Message    string   `json:"message"`
	Scheduled  int64    `json:"scheduled"`
	Started    int64    `json:"started"`
	Finished   int64    `json:"finished"`
	Latency    int64    `json:"latency"`
	Tokens     *int     `json:"tokens"`
	Mode       string   `json:"mode"`
	Task       Task     `json:"task"`
	Origins    []Origin `json:"origins"`
	HTTPStatus int      `json:"httpStatus"`
}

func marshal(v any) []byte { b, _ := json.Marshal(v); return b }

func Matches(t Task, at time.Time) bool {
	loc, err := time.LoadLocation(t.Timezone)
	if err != nil {
		return false
	}
	l := at.In(loc)
	day, clock := false, false
	for _, d := range t.Days {
		day = day || d == int(l.Weekday())
	}
	for _, v := range t.Times {
		clock = clock || v == l.Format("15:04")
	}
	if !day || !clock {
		return false
	}
	wall := l.Format("2006-01-02 15:04")
	for b := at.Add(-3 * time.Hour); b.Before(at); b = b.Add(time.Minute) {
		if b.In(loc).Format("2006-01-02 15:04") == wall {
			return false
		}
	}
	return true
}

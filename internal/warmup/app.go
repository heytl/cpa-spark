package warmup

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v3"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

type ManagementRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Query   url.Values
	Body    []byte
}
type ManagementResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}
type App struct {
	mu     sync.Mutex
	Host   Host
	Assets fs.FS
	engine *Engine
	dir    string
	closed bool
}

func (a *App) Close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return
	}
	a.closed = true
	if a.engine != nil {
		a.engine.Close()
		a.engine.store.Close()
	}
}
func (a *App) configure(raw []byte) error {
	var req struct {
		ConfigYAML    []byte `json:"config_yaml"`
		SchemaVersion uint32 `json:"schema_version"`
	}
	if json.Unmarshal(raw, &req) != nil {
		return errors.New("invalid lifecycle request")
	}
	if req.SchemaVersion < 6 {
		return errors.New("RPC schema 6 or newer required")
	}
	cfg := struct {
		Enabled  bool   `yaml:"enabled"`
		StateDir string `yaml:"state_dir"`
	}{true, "./plugins/state/cpa-spark"}
	if len(req.ConfigYAML) > 0 {
		if e := yaml.Unmarshal(req.ConfigYAML, &cfg); e != nil {
			return e
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return errors.New("plugin has quiesced; reload required")
	}
	if !cfg.Enabled {
		return errors.New("plugin disabled")
	}
	if a.engine != nil {
		if cfg.StateDir != a.dir {
			return errors.New("state_dir cannot change while loaded")
		}
		return nil
	}
	s, e := Open(cfg.StateDir)
	if e != nil {
		return e
	}
	a.dir = cfg.StateDir
	a.engine = NewEngine(s, a.Host, time.Now)
	a.engine.Start()
	return nil
}
func reply(code int, v any) ManagementResponse {
	return ManagementResponse{code, http.Header{"Content-Type": {"application/json; charset=utf-8"}, "Cache-Control": {"no-store"}, "X-Content-Type-Options": {"nosniff"}}, marshal(v)}
}
func (a *App) sidebarIcon() string {
	if a.Assets == nil {
		return ""
	}
	logo, err := fs.ReadFile(a.Assets, "sidebar-icon.svg")
	if err != nil {
		return ""
	}
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(logo)
}
func (a *App) Call(method string, raw []byte) []byte {
	var result any
	var err error
	switch method {
	case "plugin.register", "plugin.reconfigure":
		err = a.configure(raw)
		result = map[string]any{"schema_version": 6, "metadata": map[string]any{"Name": ProjectName, "Version": Version, "Author": Author, "GitHubRepository": Repository, "Logo": a.sidebarIcon(), "ConfigFields": []map[string]string{{"Name": "state_dir", "Type": "string", "Description": "SQLite state directory; persist this directory across restarts"}}}, "capabilities": map[string]bool{"management_api": true}}
	case "plugin.quiesce", "plugin.shutdown":
		a.Close()
		result = struct{}{}
	case "management.register":
		routes := []map[string]string{}
		for _, p := range []string{"state", "accounts"} {
			routes = append(routes, map[string]string{"Method": "GET", "Path": "/plugins/" + PluginID + "/" + p})
		}
		for _, p := range []string{"config", "run", "run-once", "delete-history", "cancel", "preview", "history", "history-detail", "account-timeline"} {
			routes = append(routes, map[string]string{"Method": "POST", "Path": "/plugins/" + PluginID + "/" + p})
		}
		result = map[string]any{"routes": routes, "resources": []map[string]string{{"Path": "/status", "Menu": ProjectName, "Description": ProjectDescription}, {"Path": "/app.js"}, {"Path": "/style.css"}, {"Path": "/logo.svg"}, {"Path": "/sidebar-icon.svg"}, {"Path": "/third-party-notices.txt"}}}
	case "management.handle":
		var req ManagementRequest
		if len(raw) > 2*1024*1024 || json.Unmarshal(raw, &req) != nil {
			err = errors.New("invalid management request")
		} else {
			result = a.Handle(req)
		}
	default:
		err = errors.New("unsupported plugin method")
	}
	if err != nil {
		return marshal(map[string]any{"ok": false, "error": map[string]string{"code": "warmup_error", "message": err.Error()}})
	}
	return marshal(map[string]any{"ok": true, "result": result})
}
func (a *App) Handle(r ManagementRequest) ManagementResponse {
	resource := "/v0/resource/plugins/" + PluginID + "/"
	if strings.HasPrefix(r.Path, resource) {
		if r.Method != "GET" {
			return reply(405, map[string]string{"error": "method not allowed"})
		}
		file := strings.TrimPrefix(r.Path, resource)
		if file == "status" {
			file = "index.html"
		}
		if file != "index.html" && file != "app.js" && file != "style.css" && file != "logo.svg" && file != "sidebar-icon.svg" && file != "third-party-notices.txt" {
			return reply(404, map[string]string{"error": "resource missing"})
		}
		b, e := fs.ReadFile(a.Assets, file)
		if e != nil {
			return reply(404, map[string]string{"error": "resource unavailable"})
		}
		content := "text/javascript; charset=utf-8"
		if file == "index.html" {
			content = "text/html; charset=utf-8"
		}
		if path.Ext(file) == ".css" {
			content = "text/css; charset=utf-8"
		}
		if path.Ext(file) == ".txt" {
			content = "text/plain; charset=utf-8"
		}
		if path.Ext(file) == ".svg" {
			content = "image/svg+xml"
		}
		return ManagementResponse{200, http.Header{"Content-Type": {content}, "Cache-Control": {"no-store"}, "X-Content-Type-Options": {"nosniff"}, "Content-Security-Policy": {"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self'; img-src 'self' data:; frame-ancestors 'self'; base-uri 'none'; object-src 'none'"}}, b}
	}
	a.mu.Lock()
	e := a.engine
	closed := a.closed
	a.mu.Unlock()
	if e == nil || closed {
		return reply(503, map[string]string{"error": "scheduler unavailable"})
	}
	prefix := "/v0/management/plugins/" + PluginID + "/"
	if !strings.HasPrefix(r.Path, prefix) {
		return reply(404, map[string]string{"error": "route missing"})
	}
	endpoint := strings.TrimPrefix(r.Path, prefix)
	if r.Method == "GET" {
		switch endpoint {
		case "state":
			v, err := e.Snapshot()
			if err != nil {
				return reply(500, map[string]string{"error": err.Error()})
			}
			return reply(200, v)
		case "accounts":
			v, err := e.Accounts()
			if err != nil {
				return reply(502, map[string]string{"error": err.Error()})
			}
			return reply(200, map[string]any{"accounts": v})
		}
		return reply(404, map[string]string{"error": "route missing"})
	}
	if r.Method != "POST" {
		return reply(405, map[string]string{"error": "method not allowed"})
	}
	if len(r.Body) > 1024*1024 {
		return reply(413, map[string]string{"error": "request too large"})
	}
	switch endpoint {
	case "history":
		var query HistoryQuery
		if json.Unmarshal(r.Body, &query) != nil {
			return reply(400, map[string]string{"error": "invalid history query"})
		}
		v, err := e.History(query)
		if err != nil {
			return reply(400, map[string]string{"error": err.Error()})
		}
		return reply(200, v)
	case "history-detail", "account-timeline":
		var query struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(r.Body, &query) != nil || query.ID == "" {
			return reply(400, map[string]string{"error": "record ID required"})
		}
		var v map[string]any
		var err error
		if endpoint == "history-detail" {
			v, err = e.HistoryDetail(query.ID)
		} else {
			v, err = e.AccountTimeline(query.ID)
		}
		if err != nil {
			return reply(404, map[string]string{"error": "record unavailable"})
		}
		return reply(200, v)
	case "config":
		var c Config
		if json.Unmarshal(r.Body, &c) != nil {
			return reply(400, map[string]string{"error": "invalid configuration"})
		}
		err := e.Save(c)
		if err == ErrConflict {
			return reply(409, map[string]string{"error": err.Error()})
		}
		if err != nil {
			return reply(400, map[string]string{"error": err.Error()})
		}
		v, err := e.Snapshot()
		if err != nil {
			return reply(500, map[string]string{"error": err.Error()})
		}
		return reply(200, v)
	case "run":
		var v struct {
			TaskID string `json:"taskId"`
			Key    string `json:"key"`
		}
		if json.Unmarshal(r.Body, &v) != nil {
			return reply(400, map[string]string{"error": "invalid run request"})
		}
		id, err := e.Run(v.TaskID, v.Key)
		if err != nil {
			return reply(409, map[string]string{"error": err.Error()})
		}
		return reply(202, map[string]string{"runId": id})
	case "delete-history":
		var v HistoryDelete
		if json.Unmarshal(r.Body, &v) != nil {
			return reply(400, map[string]string{"error": "invalid history deletion request"})
		}
		count, err := e.DeleteHistory(v)
		if err != nil {
			return reply(400, map[string]string{"error": err.Error()})
		}
		return reply(200, map[string]int{"count": count})
	case "run-once":
		var v struct {
			Task Task   `json:"task"`
			Key  string `json:"key"`
		}
		if json.Unmarshal(r.Body, &v) != nil {
			return reply(400, map[string]string{"error": "invalid one-off request"})
		}
		id, err := e.RunOnce(v.Task, v.Key)
		if err != nil {
			return reply(409, map[string]string{"error": err.Error()})
		}
		return reply(202, map[string]string{"runId": id})
	case "cancel":
		var v struct {
			RunID string `json:"runId"`
		}
		if json.Unmarshal(r.Body, &v) != nil || v.RunID == "" {
			return reply(400, map[string]string{"error": "runId required"})
		}
		if err := e.Cancel(v.RunID); err != nil {
			return reply(500, map[string]string{"error": err.Error()})
		}
		return reply(200, map[string]bool{"ok": true})
	case "preview":
		var v struct {
			Task Task `json:"task"`
		}
		if json.Unmarshal(r.Body, &v) != nil {
			return reply(400, map[string]string{"error": "invalid preview"})
		}
		c, err := e.store.Config()
		if err != nil {
			return reply(500, map[string]string{"error": err.Error()})
		}
		c.Tasks = []Task{v.Task}
		if err = Validate(&c); err != nil {
			return reply(400, map[string]string{"error": err.Error()})
		}
		return reply(200, map[string]any{"times": Next(v.Task, e.now(), 5), "targets": Targets(v.Task, c.Groups), "now": e.now()})
	}
	return reply(404, map[string]string{"error": "route missing"})
}

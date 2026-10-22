// Development harness: fake CPA host, loopback only, no upstream model requests.
package main

import (
	"cpa-spark/internal/warmup"
	"cpa-spark/web"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type fakeHost struct{}

func (fakeHost) Call(method string, request any, out any) error {
	var v any
	switch method {
	case "host.auth.list":
		v = map[string]any{"files": []warmup.Account{{ID: "dev-codex-1", Name: "LOCAL TEST · no upstream calls", Provider: "codex", Status: "active"}, {ID: "dev-ag-1", Name: "LOCAL TEST · Antigravity", Provider: "antigravity", Status: "active"}}}
	case "host.model.execute":
		time.Sleep(250 * time.Millisecond)
		v = warmup.ModelResponse{StatusCode: 200, Body: []byte(`{"choices":[{"message":{"content":"LOCAL TEST OK"}}],"usage":{"total_tokens":0}}`)}
	default:
		return fmt.Errorf("unsupported fake callback %s", method)
	}
	b, _ := json.Marshal(v)
	return json.Unmarshal(b, out)
}
func main() {
	dir := flag.String("state-dir", "./data/dev", "Development SQLite directory")
	listen := flag.String("listen", "127.0.0.1", "Listen interface; use 0.0.0.0 only for isolated QA")
	port := flag.Int("port", 8099, "Loopback port")
	flag.Parse()
	a := &warmup.App{Host: fakeHost{}, Assets: web.Assets}
	config, _ := json.Marshal(map[string]any{"schema_version": 6, "config_yaml": []byte("enabled: true\nstate_dir: " + *dir + "\n")})
	var env struct {
		OK    bool
		Error any
	}
	json.Unmarshal(a.Call("plugin.register", config), &env)
	if !env.OK {
		log.Fatal(env.Error)
	}
	mux := http.NewServeMux()
	// Fake CPA login for testing the same saved-session flow as the real WebUI.
	mux.HandleFunc("/management.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><title>CPA Spark 本地测试</title><h1>本地假 CPA 登录</h1><p>仅使用本地测试账号，不读取真实 OAuth，不请求上游。</p><button id="login">登录本地假 CPA</button><script>document.getElementById('login').onclick=()=>{localStorage.setItem('cli-proxy-auth',JSON.stringify({state:{apiBase:location.origin,managementKey:'dev-only',rememberPassword:true},version:0}));localStorage.setItem('isLoggedIn','true');location.href='/v0/resource/plugins/cpa-spark/status'}</script>`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/v0/resource/plugins/cpa-spark/status", 302)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v0/management/") && r.Header.Get("Authorization") != "Bearer dev-only" {
			http.Error(w, `{"error":"development key is dev-only"}`, 401)
			return
		}
		if r.URL.Path == "/v0/management/auth-files/models" || strings.HasPrefix(r.URL.Path, "/v0/management/model-definitions/") {
			provider := strings.TrimPrefix(r.URL.Path, "/v0/management/model-definitions/")
			if r.URL.Query().Get("name") == "dev-codex-1" {
				provider = "codex"
			} else if r.URL.Query().Get("name") == "dev-ag-1" {
				provider = "antigravity"
			}
			models := []map[string]any{}
			if provider == "codex" {
				models = []map[string]any{{"id": "gpt-6-luna", "thinking": map[string]any{"levels": []string{"none", "low", "medium", "high"}}}, {"id": "gpt-6-sol", "thinking": map[string]any{"levels": []string{"low", "medium", "high"}}}}
			} else if provider == "antigravity" {
				models = []map[string]any{{"id": "gemini-3-flash", "thinking": map[string]any{"min": 128, "max": 8192, "zero_allowed": true}}, {"id": "gemini-3-pro", "thinking": map[string]any{"levels": []string{"low", "high"}}}}
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"models": models})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1024*1024)
		var body []byte
		if r.Body != nil {
			var err error
			body, err = readBody(r)
			if err != nil {
				http.Error(w, "Request too large", 413)
				return
			}
		}
		response := a.Handle(warmup.ManagementRequest{Method: r.Method, Path: r.URL.Path, Headers: r.Header, Query: r.URL.Query(), Body: body})
		for k, vs := range response.Headers {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(response.StatusCode)
		w.Write(response.Body)
	})
	server := &http.Server{Addr: fmt.Sprintf("%s:%d", *listen, *port), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	go func() { <-ch; server.Close(); a.Close() }()
	log.Printf("LOCAL FAKE CPA: http://%s/management.html · fake saved-session login · no real accounts", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
	a.Close()
}

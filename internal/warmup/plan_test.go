package warmup

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

type planHost struct {
	files        []Account
	credential   map[string]any
	reads        int
	runtime      map[string]any
	runtimeReads int
}

func (h *planHost) Call(method string, request any, out any) error {
	var data any
	switch method {
	case "host.auth.list":
		data = map[string]any{"files": h.files}
	case "host.auth.get_runtime":
		h.runtimeReads++
		data = map[string]any{"auth": h.runtime}
	case "host.auth.get":
		h.reads++
		data = map[string]any{"json": h.credential}
	default:
		data = map[string]any{}
	}
	b, _ := json.Marshal(data)
	return json.Unmarshal(b, out)
}
func TestAccountPlanUsesListMetadata(t *testing.T) {
	h := &planHost{files: []Account{{ID: "p1", Provider: "codex", AccountType: "chatgpt_plus", AuthIndex: "idx"}, {ID: "p2", Provider: "antigravity", AccountType: "ultra"}}}
	e := &Engine{host: h}
	accounts, err := e.Accounts()
	if err != nil {
		t.Fatal(err)
	}
	if accounts[0].Plan != "ChatGPT Plus" || accounts[1].Plan != "Google AI Ultra" {
		t.Fatalf("unexpected plans: %+v", accounts)
	}
	if h.reads != 0 {
		t.Fatal("needless credential reads")
	}
}
func TestAccountPlanReadsOnlyExplicitMetadata(t *testing.T) {
	h := &planHost{files: []Account{{ID: "a", Provider: "antigravity", AuthIndex: "idx"}}, credential: map[string]any{"subscription": map[string]any{"tier": "pro"}, "access_token": "secret"}}
	accounts, err := (&Engine{host: h}).Accounts()
	if err != nil {
		t.Fatal(err)
	}
	if accounts[0].Plan != "Google AI Pro" || h.reads != 1 {
		t.Fatalf("unexpected: %+v %d", accounts, h.reads)
	}
	b, _ := json.Marshal(accounts)
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "access_token") {
		t.Fatal("secret exposed")
	}
}
func TestAccountPlanJWTClaimAndUnknown(t *testing.T) {
	claims, _ := json.Marshal(map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "pro"}})
	jwt := "e30." + base64.RawURLEncoding.EncodeToString(claims) + ".sig"
	if p := credentialPlan(mustJSON(map[string]string{"id_token": jwt}), "codex"); p != "ChatGPT Pro" {
		t.Fatal(p)
	}
	if p := credentialPlan(mustJSON(map[string]any{"access_token": "secret", "type": "antigravity"}), "antigravity"); p != "" {
		t.Fatal(p)
	}
	h := &planHost{files: []Account{{ID: "a", Provider: "antigravity", AuthIndex: "idx"}}, credential: map[string]any{"type": "antigravity"}}
	a, err := (&Engine{host: h}).Accounts()
	if err != nil || a[0].Plan != "未识别" {
		t.Fatalf("unexpected: %+v %v", a, err)
	}
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

func TestCPAAuthFilePlanFieldsAndNestedTokens(t *testing.T) {
	h := &planHost{files: []Account{
		{ID: "codex1", Provider: "codex", PlanType: "plus", AuthIndex: "idx1"},
		{ID: "anti1", Provider: "antigravity", Tier: "ultra", AuthIndex: "idx2"},
	}}
	actual, err := (&Engine{host: h}).Accounts()
	if err != nil {
		t.Fatal(err)
	}
	if actual[0].Plan != "ChatGPT Plus" || actual[1].Plan != "Google AI Ultra" || h.runtimeReads != 0 || h.reads != 0 {
		t.Fatalf("bad list mapping: %+v / %d %d", actual, h.runtimeReads, h.reads)
	}
	if plan := credentialPlan(mustJSON(map[string]any{"account_type": "oauth", "tokens": map[string]any{"id_token": map[string]any{"https://api.openai.com/auth": map[string]string{"chatgpt_plan_type": "team"}}}}), "codex"); plan != "ChatGPT Team" {
		t.Fatal(plan)
	}
	if plan := credentialPlan(mustJSON(map[string]any{"metadata": map[string]any{"tier": "pro"}}), "antigravity"); plan != "Google AI Pro" {
		t.Fatal(plan)
	}
}
func TestRuntimeFallbackAndNoPlanGuessing(t *testing.T) {
	h := &planHost{files: []Account{{ID: "a", Provider: "codex", AuthIndex: "idx", AccountType: "oauth"}}, runtime: map[string]any{"account_type": "chatgpt_pro"}, credential: map[string]any{"access_token": "sensitive"}}
	actual, err := (&Engine{host: h}).Accounts()
	if err != nil || actual[0].Plan != "ChatGPT Pro" || h.runtimeReads != 1 || h.reads != 0 {
		t.Fatalf("bad runtime: %+v / %d %d / %v", actual, h.runtimeReads, h.reads, err)
	}
	if displayPlan("codex", "oauth") != "" || displayPlan("codex", "unknown") != "" || displayPlan("antigravity", "supported") != "" {
		t.Fatal("invented plan from non-plan field")
	}
}
func TestPhysicalFileJWTIsUsedWhenRuntimeHasNoPlan(t *testing.T) {
	claims, _ := json.Marshal(map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_plan_type": "prolite"}})
	jwt := "e30." + base64.RawURLEncoding.EncodeToString(claims) + ".signature"
	h := &planHost{files: []Account{{ID: "a", Provider: "codex", AuthIndex: "idx", AccountType: "oauth"}}, credential: map[string]any{"id_token": jwt, "refresh_token": "secret"}}
	actual, err := (&Engine{host: h}).Accounts()
	if err != nil || actual[0].Plan != "ChatGPT Pro Lite" || h.reads != 1 {
		t.Fatalf("not decoded: %+v / %v", actual, err)
	}
	b, _ := json.Marshal(actual)
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "refresh_token") {
		t.Fatal("credential token leaked")
	}
}

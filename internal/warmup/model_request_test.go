package warmup

import (
	"encoding/json"
	"testing"
)

func TestThinkingRequest(t *testing.T) {
	for _, tc := range []struct{ provider, effort, model, field string }{
		{"codex", "none", "test-model", "none"},
		{"codex", "medium", "test-model", "medium"},
		{"antigravity", "low", "test-model(low)", ""},
		{"antigravity", "budget:0", "test-model(0)", ""},
		{"antigravity", "budget:128", "test-model(128)", ""},
		{"codex", "budget:512", "test-model(512)", ""},
		{"antigravity", "", "test-model", ""},
	} {
		t.Run(tc.provider+"/"+tc.effort, func(t *testing.T) {
			x := Execution{Model: "test-model", AuthID: "exact-account", Provider: tc.provider, Task: Task{Effort: tc.effort}}
			r := buildModelRequest(x)
			var body map[string]any
			if err := json.Unmarshal(r.Body, &body); err != nil {
				t.Fatal(err)
			}
			if r.Model != tc.model || body["model"] != tc.model || r.AuthID != x.AuthID || r.ForcedProvider != tc.provider {
				t.Fatalf("incorrect pinned request: %+v body=%v", r, body)
			}
			if tc.field != "" && body["reasoning_effort"] != tc.field {
				t.Fatalf("missing reasoning effort: %v", body)
			}
			if tc.field == "" && body["reasoning_effort"] != nil {
				t.Fatalf("unexpected reasoning effort: %v", body)
			}
		})
	}
}

func TestEffortValidation(t *testing.T) {
	for _, v := range []string{"", "none", "minimal", "low", "medium", "high", "xhigh", "max", "auto", "budget:0", "budget:128"} {
		if !validEffort(v) {
			t.Errorf("rejected supported effort %q", v)
		}
	}
	for _, v := range []string{"budget:-1", "budget:+1", "budget:01", "budget:1000001", "budget:0)(high", "unknown"} {
		if validEffort(v) {
			t.Errorf("accepted invalid effort %q", v)
		}
	}
}

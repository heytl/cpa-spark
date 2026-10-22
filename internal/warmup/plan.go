package warmup

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// displayPlan accepts explicit subscription values only, never model availability,
// a credential's login method (such as "oauth"), or guessed file-name signals.
func displayPlan(provider, candidate string) string {
	p := strings.ToLower(strings.TrimSpace(candidate))
	for _, prefix := range []string{"chatgpt ", "chatgpt_", "chatgpt-", "google ai ", "google_ai_", "google-ai-", "antigravity_"} {
		p = strings.TrimPrefix(p, prefix)
	}
	p = strings.ReplaceAll(strings.ReplaceAll(p, "-", "_"), " ", "_")
	switch provider {
	case "codex":
		switch p {
		case "free", "plus", "pro", "team", "business", "enterprise", "edu", "go", "k12":
			return "ChatGPT " + strings.ToUpper(p[:1]) + p[1:]
		case "prolite", "pro_lite":
			return "ChatGPT Pro Lite"
		}
	case "antigravity":
		switch p {
		case "free", "individual", "basic":
			return "Individual"
		case "plus":
			return "Google AI Plus"
		case "pro":
			return "Google AI Pro"
		case "ultra":
			return "Google AI Ultra"
		case "business":
			return "Business"
		}
	}
	return ""
}

func accountPlan(host Host, a Account) string {
	// The list callback may return explicit plan metadata on some CPA versions.
	for _, value := range []string{a.PlanType, a.ChatGPTPlanType, a.Tier, a.Plan, a.AccountType} {
		if plan := displayPlan(a.Provider, value); plan != "" {
			return plan
		}
	}
	if a.AuthIndex == "" {
		return "未识别"
	}
	request := map[string]string{"auth_index": a.AuthIndex}
	// The supported runtime callback is useful for memory-only credentials.
	// It returns {"auth": HostAuthFileEntry} and normally has no plan field.
	var runtime struct {
		Auth json.RawMessage `json:"auth"`
	}
	if host.Call("host.auth.get_runtime", request, &runtime) == nil {
		if plan := credentialPlan(runtime.Auth, a.Provider); plan != "" {
			return plan
		}
	}
	// The physical JSON is needed for Codex plan_type and Antigravity tier.
	// NEVER return, cache, serialize or log the file's OAuth tokens.
	var credential struct {
		JSON json.RawMessage `json:"json"`
	}
	if host.Call("host.auth.get", request, &credential) == nil {
		if plan := credentialPlan(credential.JSON, a.Provider); plan != "" {
			return plan
		}
	}
	return "未识别"
}

func credentialPlan(raw json.RawMessage, provider string) string {
	if len(raw) == 0 || len(raw) > 1024*1024 {
		return ""
	}
	var data map[string]json.RawMessage
	if json.Unmarshal(raw, &data) != nil {
		var wrapped string
		if json.Unmarshal(raw, &wrapped) != nil || len(wrapped) > 1024*1024 || json.Unmarshal([]byte(wrapped), &data) != nil {
			return ""
		}
	}
	return lookupPlan(data, provider, 0)
}

func lookupPlan(data map[string]json.RawMessage, provider string, depth int) string {
	if depth > 5 || data == nil {
		return ""
	}
	for _, key := range []string{"chatgpt_plan_type", "plan_type", "subscription_plan", "subscription_tier", "plan", "tier"} {
		var value string
		if json.Unmarshal(data[key], &value) == nil {
			if plan := displayPlan(provider, value); plan != "" {
				return plan
			}
		}
	}
	// account_type describes OAuth/API-key credentials in CPA, not necessarily a
	// subscription. Only known plan words pass the strict normalization above.
	var accountType string
	if json.Unmarshal(data["account_type"], &accountType) == nil {
		if plan := displayPlan(provider, accountType); plan != "" {
			return plan
		}
	}
	for _, key := range []string{"subscription", "account", "metadata", "attributes", "auth", "auth_info", "https://api.openai.com/auth", "credentials", "tokens", "user", "profile", "id_token"} {
		var child map[string]json.RawMessage
		if json.Unmarshal(data[key], &child) == nil && child != nil {
			if plan := lookupPlan(child, provider, depth+1); plan != "" {
				return plan
			}
		}
	}
	// Some Codex credentials store the explicit plan only in a JWT claim.
	// Decode locally without trusting it as an authorization decision.
	if provider == "codex" {
		for _, key := range []string{"id_token", "access_token"} {
			var token string
			if json.Unmarshal(data[key], &token) != nil {
				continue
			}
			parts := strings.Split(token, ".")
			if len(parts) != 3 || len(parts[1]) > 65536 {
				continue
			}
			claims, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				continue
			}
			var values map[string]json.RawMessage
			if json.Unmarshal(claims, &values) == nil {
				if plan := lookupPlan(values, provider, depth+1); plan != "" {
					return plan
				}
			}
		}
	}
	return ""
}

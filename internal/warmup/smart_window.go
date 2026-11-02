package warmup

import (
    "encoding/json"
    "errors"
    "net/http"
    "strconv"
    "strings"
    "time"
)

const (
    windowSafetyMargin = 5 * time.Second
    codexUsageURL = "https://chatgpt.com/backend-api/wham/usage"
    antigravityUsageURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary"
    antigravityCodeAssistURL = "https://daily-cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"
)

var errQuotaUnknown = errors.New("upstream quota window unavailable")

// WindowObservation is an observation, not a promise that sending a model
// request will activate a fresh quota window. Never store auth material here.
type WindowObservation struct {
    ResetAt int64   `json:"resetAt,omitempty"`
    CheckedAt int64 `json:"checkedAt,omitempty"`
    Bucket string   `json:"bucket,omitempty"`
    Provider string `json:"provider,omitempty"`
    Source string   `json:"source,omitempty"`
}

func readRawObject(raw json.RawMessage) map[string]any {
    var value map[string]any
    if json.Unmarshal(raw, &value) == nil && value != nil { return value }
    var inner string
    if json.Unmarshal(raw, &inner) == nil && len(inner) < 1024*1024 && json.Unmarshal([]byte(inner), &value) == nil { return value }
    return nil
}

func obj(v any) map[string]any {
    o, _ := v.(map[string]any)
    return o
}
func textVal(v any) string {
    if s, ok := v.(string); ok { return strings.TrimSpace(s) }
    return ""
}
func firstText(m map[string]any, keys ...string) string {
    for _, k := range keys { if s := textVal(m[k]); s != "" { return s } }
    return ""
}
func floatVal(v any) float64 {
    switch n := v.(type) {
    case float64: return n
    case json.Number: f,_ := n.Float64(); return f
    case string: f,_ := strconv.ParseFloat(n,64); return f
    }
    return 0
}
func parseResetInstant(v any, now time.Time) int64 {
    if v == nil { return 0 }
    if s := textVal(v); s != "" {
        if at, err := time.Parse(time.RFC3339Nano, s); err == nil { return at.UnixMilli() }
        if f, err := strconv.ParseFloat(s,64); err == nil { return numericReset(f) }
    }
    return numericReset(floatVal(v))
}
func numericReset(n float64) int64 {
    if n < 1000000000 || n > 100000000000000 { return 0 }
    if n < 100000000000 { return int64(n * 1000) }
    return int64(n)
}

func findCredentialValue(value any, key string, depth int) string {
    if depth > 5 { return "" }
    m := obj(value)
    if m == nil { return "" }
    if text := textVal(m[key]); text != "" { return text }
    for _, childKey := range []string{"tokens","credentials","auth","oauth","metadata","account"} {
        if s := findCredentialValue(m[childKey],key,depth+1); s != "" { return s }
    }
    return ""
}

func (e *Engine) observeWindow(account Account, model string) (WindowObservation, error) {
    if account.AuthIndex == "" { return WindowObservation{}, errQuotaUnknown }
    var credential struct { JSON json.RawMessage `json:"json"` }
    if err := e.host.Call("host.auth.get", map[string]string{"auth_index": account.AuthIndex}, &credential); err != nil {
        return WindowObservation{}, errQuotaUnknown
    }
    raw := readRawObject(credential.JSON)
    token := findCredentialValue(raw,"access_token",0)
    if token == "" { return WindowObservation{}, errQuotaUnknown }
    headers := http.Header{"Authorization": {"Bearer "+token}}
    now := e.now()
    var response ModelResponse
    var err error
    switch account.Provider {
    case "codex":
        headers.Set("Content-Type","application/json")
        headers.Set("User-Agent","codex_cli_rs")
        if id := firstText(raw,"chatgpt_account_id","account_id"); id != "" { headers.Set("ChatGPT-Account-ID",id) }
        err = e.host.Call("host.http.do", map[string]any{"method":"GET","url":codexUsageURL,"headers":headers}, &response)
    case "antigravity":
        headers.Set("Content-Type","application/json")
        headers.Set("User-Agent","antigravity/hub/2.23.0 darwin/arm64")
        project := account.ProjectID
        if project == "" { project = firstText(raw,"project_id","projectId") }
        if project == "" {
            var lookup ModelResponse
            body := []byte(`{"metadata":{"ideType":"ANTIGRAVITY","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`)
            if e.host.Call("host.http.do",map[string]any{"method":"POST","url":antigravityCodeAssistURL,"headers":headers,"body":body},&lookup)==nil && lookup.StatusCode>=200 && lookup.StatusCode<300 {
                m:=readRawObject(lookup.Body)
                project=firstText(obj(m["cloudaicompanionProject"]),"id")
            }
        }
        if project == "" { project = "aicode-consumers" }
        body,_ := json.Marshal(map[string]string{"project":project})
        err = e.host.Call("host.http.do",map[string]any{"method":"POST","url":antigravityUsageURL,"headers":headers,"body":body},&response)
    default:
        return WindowObservation{}, errQuotaUnknown
    }
    // Tokens and headers are scoped to this call and never persisted or logged.
    if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 || len(response.Body) == 0 || len(response.Body) > 1024*1024 {
        return WindowObservation{}, errQuotaUnknown
    }
    var result WindowObservation
    switch account.Provider {
    case "codex":
        result,err = parseCodexWindow(response.Body,now)
    case "antigravity":
        result,err = parseAntigravityWindow(response.Body,model,now)
    }
    if err != nil || result.ResetAt <= 0 { return WindowObservation{}, errQuotaUnknown }
    result.CheckedAt=now.UnixMilli()
    result.Provider=account.Provider
    result.Source="upstream"
    return result,nil
}

func parseCodexWindow(body []byte, now time.Time) (WindowObservation,error) {
    m:=readRawObject(body)
    rate:=obj(m["rate_limit"])
    if rate==nil { rate=obj(m["rateLimit"]) }
    for _,field:=range []string{"primary_window","secondary_window","primaryWindow","secondaryWindow"} {
        w:=obj(rate[field])
        if w==nil { continue }
        length:=floatVal(w["limit_window_seconds"])
        if length==0 { length=floatVal(w["limitWindowSeconds"]) }
        if length!=18000 { continue }
        reset:=parseResetInstant(w["reset_at"],now)
        if reset==0 { reset=parseResetInstant(w["resetAt"],now) }
        if reset==0 {
            seconds:=floatVal(w["reset_after_seconds"])
            if seconds==0 {seconds=floatVal(w["resetAfterSeconds"])}
            if seconds>0 && seconds<60*60*24*30 {reset=now.Add(time.Duration(seconds*float64(time.Second))).UnixMilli()}
        }
        if reset>0 {return WindowObservation{ResetAt:reset,Bucket:"codex-5h"},nil}
    }
    return WindowObservation{},errQuotaUnknown
}

func antigravityFamily(model string) string {
    m:=strings.ToLower(strings.TrimSpace(model))
    switch {
    case strings.Contains(m,"gemini"):return "gemini"
    case strings.Contains(m,"claude"),strings.Contains(m,"gpt"):return "third-party"
    default:return ""
    }
}
func parseAntigravityWindow(body []byte, model string, now time.Time) (WindowObservation,error) {
    root:=readRawObject(body)
    groups,_:=root["groups"].([]any)
    family:=antigravityFamily(model)
    if family=="" || len(groups)==0 {return WindowObservation{},errQuotaUnknown}
    type candidate struct{reset int64; bucket string; label string}
    candidates:=[]candidate{}
    for _,g:=range groups {
        group:=obj(g)
        name:=strings.ToLower(firstText(group,"displayName","display_name","description"))
        buckets,_:=group["buckets"].([]any)
        for _,b:=range buckets {
            bucket:=obj(b)
            window:=strings.ToLower(firstText(bucket,"window"))
            if window!="5h" && window!="five-hour" && window!="five_hour" {continue}
            bucketID:=firstText(bucket,"bucketId","bucket_id")
            label:=strings.ToLower(name+" "+bucketID+" "+firstText(bucket,"displayName","display_name","description"))
            belongs:=(family=="gemini" && strings.Contains(label,"gemini")) || (family=="third-party" && (strings.Contains(label,"claude")||strings.Contains(label,"gpt")||strings.Contains(label,"3p")))
            if !belongs {continue}
            reset:=parseResetInstant(bucket["resetTime"],now)
            if reset==0 {reset=parseResetInstant(bucket["reset_time"],now)}
            if reset>0 {candidates=append(candidates,candidate{reset,bucketID,label})}
        }
    }
    if len(candidates)==0 {return WindowObservation{},errQuotaUnknown}
    if len(candidates)>1 {
        // A quota-group name alone is not sufficient to pick one of several
        // 5-hour buckets. Only a unique exact model/bucket match is safe.
        desired:=strings.ToLower(model)
        matches:=[]candidate{}
        for _,c:=range candidates {
            if c.bucket!="" && (strings.Contains(desired,strings.ToLower(c.bucket)) || strings.Contains(strings.ToLower(c.bucket),desired)) {
                matches=append(matches,c)
            }
        }
        if len(matches)!=1 {return WindowObservation{},errors.New("ambiguous model quota bucket")}
        candidates=matches
    }
    return WindowObservation{ResetAt:candidates[0].reset,Bucket:family+"-5h"},nil
}

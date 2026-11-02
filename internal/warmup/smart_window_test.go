package warmup

import (
 "encoding/json"
 "strings"
 "sync"
 "sync/atomic"
 "testing"
 "time"
)

func TestParseCodexLiveFiveHour(t *testing.T) {
 now:=time.Date(2026,11,2,11,0,0,0,time.UTC)
 body:=[]byte(`{"rate_limit":{"primary_window":{"limit_window_seconds":18000,"reset_at":1793617230},"secondary_window":{"limit_window_seconds":604800,"reset_at":1794000000}}}`)
 got,err:=parseCodexWindow(body,now)
 if err!=nil || got.Bucket!="codex-5h" || got.ResetAt!=1793617230000 {t.Fatalf("%+v %v",got,err)}
 if _,err:=parseCodexWindow([]byte(`{"rate_limit":{"primary_window":{"limit_window_seconds":604800,"reset_at":1794000000}}}`),now);err==nil {t.Fatal("weekly quota must not masquerade as five-hour window")}
}
func TestAntigravityExactQuotaBucket(t *testing.T){
 now:=time.Now()
 body:=[]byte(`{"groups":[{"displayName":"Gemini Models","buckets":[{"window":"5h","bucketId":"gemini-primary","resetTime":"2026-11-02T11:01:00Z","remainingFraction":0.99}]},{"displayName":"Claude Models","buckets":[{"window":"5h","bucketId":"claude-primary","resetTime":"2026-11-02T13:00:00Z","remainingFraction":0.5}]}]}`)
 g,e:=parseAntigravityWindow(body,"gemini-3-flash",now)
 if e!=nil || g.Bucket!="gemini-5h" {t.Fatalf("%+v %v",g,e)}
 c,e:=parseAntigravityWindow(body,"claude-sonnet",now)
 if e!=nil || c.Bucket!="third-party-5h"|| c.ResetAt==g.ResetAt {t.Fatalf("%+v %v",c,e)}
 if _,e:=parseAntigravityWindow(body,"mystery-model",now);e==nil {t.Fatal("unknown family should never be inferred")}
}
type smartTestHost struct {
 mu sync.Mutex
 now *atomic.Int64
 provider string
 resetBefore int64
 resetAfter int64
 calls int
}
func (h *smartTestHost) Call(method string,in any,out any) error {
 h.mu.Lock()
 defer h.mu.Unlock()
 var payload any
 switch method {
 case "host.auth.list":
  payload=map[string]any{"files":[]Account{{ID:"a",AuthIndex:"idx",Provider:h.provider,Status:"active",ProjectID:"project-123"}}}
 case "host.auth.get_runtime":
  payload=map[string]any{"auth":map[string]any{"id":"a","auth_index":"idx","provider":h.provider}}
 case "host.auth.get":
  payload=map[string]any{"json":json.RawMessage(`{"access_token":"placeholder","project_id":"project-123"}`)}
 case "host.http.do":
  request:=in.(map[string]any)
  url:=request["url"].(string)
  reset:=h.resetBefore
  if h.calls>0 {reset=h.resetAfter}
  if strings.Contains(url,"wham/usage") {
   payload=ModelResponse{StatusCode:200,Body:marshal(map[string]any{"rate_limit":map[string]any{"primary_window":map[string]any{"limit_window_seconds":18000,"reset_at":reset/1000}}})}
  }else if strings.Contains(url,"retrieveUserQuotaSummary") {
   payload=ModelResponse{StatusCode:200,Body:marshal(map[string]any{"groups":[]any{map[string]any{"displayName":"Gemini Models","buckets":[]any{map[string]any{"window":"5h","bucketId":"gemini","resetTime":time.UnixMilli(reset).UTC().Format(time.RFC3339),"remainingFraction":0.5}}}}})}
  }else {payload=ModelResponse{StatusCode:404}}
 case "host.model.execute":
  h.calls++
  payload=ModelResponse{StatusCode:200,Body:[]byte(`{"choices":[{"message":{"content":"OK"}}]}`)}
 default:
  return nil
 }
 return json.Unmarshal(marshal(payload),out)
}
func smartEngine(t *testing.T,provider string,now time.Time,before,after time.Time)(*Engine,*Store,*smartTestHost,*atomic.Int64){
 t.Helper()
 clock:=new(atomic.Int64);clock.Store(now.UnixMilli())
 host:=&smartTestHost{provider:provider,now:clock,resetBefore:before.UnixMilli(),resetAfter:after.UnixMilli()}
 store,err:=Open(t.TempDir());if err!=nil {t.Fatal(err)}
 engine:=NewEngine(store,host,func()time.Time{return time.UnixMilli(clock.Load())})
 engine.Start()
 t.Cleanup(func(){engine.Close();store.Close()})
 task:=taskFixture("smart");task.Provider=provider;task.Model=map[string]string{"codex":"gpt-5.6-codex","antigravity":"gemini-3-flash"}[provider]
 mustSave(t,store,task)
 return engine,store,host,clock
}
func TestFixedWaitsUntilRealResetAndVerifies(t *testing.T){
 start:=time.Date(2026,11,2,11,0,0,0,time.UTC)
 for _,provider:=range []string{"codex","antigravity"} {
  t.Run(provider,func(t *testing.T){
   e,s,h,clock:=smartEngine(t,provider,start,start.Add(30*time.Second),start.Add(5*time.Hour+30*time.Second))
   if _,err:=e.Run("smart","test-wait-123");err!=nil {t.Fatal(err)}
   e.Tick()
   waitFor(t,func()bool{xs,_:=s.Executions();return len(xs)==1&&xs[0].Status=="waiting"})
   h.mu.Lock();sent:=h.calls;h.mu.Unlock();if sent!=0 {t.Fatalf("sent early: %d",sent)}
   clock.Store(start.Add(36*time.Second).UnixMilli())
   e.Tick()
   waitFor(t,func()bool{xs,_:=s.Executions();return xs[0].Status=="success"})
   xs,_:=s.Executions()
   h.mu.Lock();sent=h.calls;h.mu.Unlock();if sent!=1||xs[0].WindowStatus!="confirmed"||xs[0].Dispatched<=start.UnixMilli(){t.Fatalf("unexpected result %+v calls=%d",xs[0],sent)}
  })
 }
}
func TestFixedSkipsFarFutureWindow(t *testing.T){
 start:=time.Date(2026,11,2,11,0,0,0,time.UTC)
 e,s,h,_:=smartEngine(t,"codex",start,start.Add(45*time.Minute),start.Add(5*time.Hour))
 e.Run("smart","far-future")
 e.Tick()
 waitFor(t,func()bool{xs,_:=s.Executions();return xs[0].Status=="skipped"})
 xs,_:=s.Executions()
 h.mu.Lock();sent:=h.calls;h.mu.Unlock();if xs[0].WindowStatus!="existing"||sent!=0 {t.Fatalf("%+v calls=%d",xs[0],sent)}
}
func TestRollingSavesNextDue(t *testing.T){
 start:=time.Date(2026,11,2,11,0,0,0,time.UTC)
 e,s,h,_:=smartEngine(t,"codex",start,start.Add(-10*time.Second),start.Add(5*time.Hour))
 c,_:=s.Config();c.Tasks[0].Strategy="rolling";if err:=e.Save(c);err!=nil{t.Fatal(err)}
 e.Run("smart","roll-now")
 e.Tick()
 waitFor(t,func()bool{xs,_:=s.Executions();return xs[0].Status=="success"})
 // A manual warmup on a rolling task should not invent an extra automatic
 // rolling cycle; the first scheduled bootstrap will establish its cursor.
 if p,found,err:=s.Rolling("smart","a");err!=nil||!found||p.NextDue<=start.UnixMilli(){t.Fatalf("plan=%+v found=%v err=%v calls=%d",p,found,err,h.calls)}
}

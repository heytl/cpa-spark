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
 quotaHTTPStatus int
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
  if h.quotaHTTPStatus>0 {
   payload=QuotaHTTPResponse{StatusCode:h.quotaHTTPStatus}
  }else if strings.Contains(url,"wham/usage") {
   payload=QuotaHTTPResponse{StatusCode:200,Body:marshal(map[string]any{"rate_limit":map[string]any{"primary_window":map[string]any{"limit_window_seconds":18000,"reset_at":reset/1000}}})}
  }else if strings.Contains(url,"retrieveUserQuotaSummary") {
   payload=QuotaHTTPResponse{StatusCode:200,Body:marshal(map[string]any{"groups":[]any{map[string]any{"displayName":"Gemini Models","buckets":[]any{map[string]any{"window":"5h","bucketId":"gemini","resetTime":time.UnixMilli(reset).UTC().Format(time.RFC3339),"remainingFraction":0.5}}}}})}
  }else {payload=QuotaHTTPResponse{StatusCode:404}}
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

func TestRollingPreviewUsesFiveHourIntervalsAndPerAccountCursor(t *testing.T) {
    now:=time.Date(2026,11,2,14,35,0,0,time.UTC)
    e,store,_,_:=smartEngine(t,"codex",now,now.Add(time.Hour),now.Add(5*time.Hour))
    cfg,_:=store.Config()
    task:=cfg.Tasks[0]
    task.Strategy="rolling"
    task.Times=[]string{"00:30"}
    task.Days=[]int{0,1,2,3,4,5,6}
    cfg.Tasks[0]=task
    if err:=e.Save(cfg);err!=nil {t.Fatal(err)}
    times,plans,err:=e.previewRolling(task,cfg.Groups,now)
    if err!=nil||len(times)!=5||len(plans)!=1 {t.Fatalf("unexpected preview %v %v %v",times,plans,err)}
    loc,_:=time.LoadLocation("Asia/Shanghai")
    if first:=times[0].In(loc).Format("2006-01-02 15:04");first!="2026-11-03 00:30" {t.Fatalf("incorrect bootstrap: %s",first)}
    for i:=1;i<5;i++ {
        if gap:=times[i].Sub(times[i-1]);gap!=5*time.Hour+windowSafetyMargin {
            t.Fatalf("gap[%d] was %s (daily repetition regression)",i,gap)
        }
    }
    due:=now.Add(2*time.Hour).UnixMilli()
    if err=store.SaveRolling(RollingPlan{TaskID:task.ID,AuthID:"a",NextDue:due,Source:"upstream"});err!=nil {t.Fatal(err)}
    times,plans,err=e.previewRolling(task,cfg.Groups,now)
    if err!=nil||times[0].UnixMilli()!=due||plans[0].Source!="upstream" {t.Fatalf("real per-account plan ignored: %v %v %v",times,plans,err)}
    // Turning off scheduling deletes the saved continuous cursor, rather than
    // allowing a stale queued occurrence to reappear on re-enable.
    cfg,err=store.Config();if err!=nil {t.Fatal(err)}
    cfg.Tasks[0].Enabled=false
    cfg.Tasks[0].Revision++
    if err=e.Save(cfg);err!=nil {t.Fatal(err)}
    if p,exists,err:=store.Rolling(task.ID,"a");err!=nil||exists {t.Fatalf("disabled cursor retained: %+v %v %v",p,exists,err)}
}

func TestSnapshotIncludesPerAccountRollingSchedule(t *testing.T){
 now:=time.Date(2026,11,2,11,0,0,0,time.UTC)
 e,store,_,_:=smartEngine(t,"codex",now,now.Add(time.Hour),now.Add(5*time.Hour))
 cfg,err:=store.Config()
 if err!=nil {t.Fatal(err)}
 cfg.Tasks[0].Strategy="rolling"
 cfg.Tasks[0].Revision++
 if err=e.Save(cfg);err!=nil {t.Fatal(err)}
 next:=now.Add(5*time.Hour+5*time.Second).UnixMilli()
 if err=store.SaveRolling(RollingPlan{TaskID:"smart",AuthID:"a",NextDue:next,Source:"upstream"});err!=nil{t.Fatal(err)}
 snap,err:=e.Snapshot()
 if err!=nil{t.Fatal(err)}
 plans,ok:=snap["rollingPlans"].(map[string][]RollingPlan)
 if !ok||len(plans["smart"])!=1||plans["smart"][0].NextDue!=next {t.Fatalf("incorrect snapshot: %+v",snap["rollingPlans"])}
}

func TestQuotaHTTPResponseUsesOfficialPascalCaseWireFormat(t *testing.T) {
    // host.http.do marshals pluginapi.HTTPResponse with Go exported field
    // names, unlike host.model.execute's snake_case RPC response.
    original:=QuotaHTTPResponse{StatusCode:200,Body:[]byte(`{"rate_limit":{}}`)}
    raw:=marshal(original)
    if !strings.Contains(string(raw),`"StatusCode":200`) {
        t.Fatalf("unexpected host.http.do JSON: %s",raw)
    }
    var quota QuotaHTTPResponse
    if err:=json.Unmarshal(raw,&quota);err!=nil||quota.StatusCode!=200||len(quota.Body)==0 {
        t.Fatalf("valid host HTTP response misread: %+v, %v",quota,err)
    }
    var old ModelResponse
    if err:=json.Unmarshal(raw,&old);err!=nil {
        t.Fatal(err)
    }
    if old.StatusCode!=0 { t.Fatalf("regression assertion needs old snake_case to remain incompatible: %+v",old) }
}

func TestRollingWaitDoesNotDuplicateOrStayOnStaleDue(t *testing.T) {
    start:=time.Date(2026,11,3,7,30,0,0,time.UTC)
    firstReset:=start.Add(30*time.Minute)
    secondReset:=start.Add(5*time.Hour+30*time.Minute)
    engine,store,host,clock:=smartEngine(t,"codex",start,firstReset,secondReset)
    cfg,err:=store.Config()
    if err!=nil{t.Fatal(err)}
    cfg.Tasks[0].Strategy="rolling"
    cfg.Tasks[0].Revision++
    if err=engine.Save(cfg);err!=nil{t.Fatal(err)}
    if err=store.SaveRolling(RollingPlan{TaskID:"smart",AuthID:"a",NextDue:start.UnixMilli(),Source:"upstream"});err!=nil{t.Fatal(err)}
    engine.Tick()
    waitFor(t,func()bool{xs,_:=store.Executions();return len(xs)==1&&xs[0].Status=="waiting"})
    xs,err:=store.Executions()
    if err!=nil{t.Fatal(err)}
    if xs[0].WaitUntil!=firstReset.Add(windowSafetyMargin).UnixMilli(){t.Fatalf("incorrect wait: %+v",xs[0])}
    cursor,ok,err:=store.Rolling("smart","a")
    if err!=nil||!ok||cursor.NextDue!=xs[0].WaitUntil{t.Fatalf("waiting cursor not moved: %+v %v %v",cursor,ok,err)}
    for n:=1;n<10;n++{
        clock.Store(start.Add(time.Duration(n)*time.Second).UnixMilli())
        engine.Tick()
    }
    xs,err=store.Executions()
    if err!=nil||len(xs)!=1{t.Fatalf("duplicate rolling batch while waiting: %d %v",len(xs),err)}
    // Exactly as a real external request would do, the provider has already
    // advanced to the NEXT five-hour window when the wait expires.
    host.mu.Lock()
    host.resetBefore=secondReset.UnixMilli()
    host.mu.Unlock()
    clock.Store(firstReset.Add(10*time.Second).UnixMilli())
    engine.Tick()
    waitFor(t,func()bool{items,_:=store.Executions();return len(items)==1&&items[0].Status=="skipped"})
    cursor,ok,err=store.Rolling("smart","a")
    if err!=nil||!ok||cursor.NextDue!=secondReset.Add(windowSafetyMargin).UnixMilli(){t.Fatalf("stale cursor after external advancement: %+v %v %v",cursor,ok,err)}
    engine.Tick()
    xs,err=store.Executions()
    if err!=nil||len(xs)!=1{t.Fatalf("new run spawned from stale due: %d %v",len(xs),err)}
    host.mu.Lock()
    calls:=host.calls
    host.mu.Unlock()
    if calls!=0{t.Fatalf("duplicate token-consuming model request: %d",calls)}
}
func TestManualRollingDoesNotWaitFiveHours(t *testing.T) {
    start:=time.Date(2026,11,3,7,30,0,0,time.UTC)
    reset:=start.Add(5*time.Hour)
    engine,store,host,_:=smartEngine(t,"codex",start,reset,reset)
    cfg,err:=store.Config()
    if err!=nil{t.Fatal(err)}
    cfg.Tasks[0].Strategy="rolling"
    cfg.Tasks[0].Revision++
    if err=engine.Save(cfg);err!=nil{t.Fatal(err)}
    if _,err=engine.Run("smart","manual-long-wait-guard");err!=nil{t.Fatal(err)}
    engine.Tick()
    waitFor(t,func()bool{xs,_:=store.Executions();return len(xs)==1&&xs[0].Status=="skipped"})
    xs,err:=store.Executions()
    if err!=nil{t.Fatal(err)}
    if xs[0].WindowStatus!="existing"||xs[0].WaitUntil!=0{t.Fatalf("manual request left waiting for hours: %+v",xs[0])}
    host.mu.Lock()
    calls:=host.calls
    host.mu.Unlock()
    if calls!=0{t.Fatalf("manual action consumed tokens inside active window: %d",calls)}
}
func TestWaitingRunIsNotReportedAsRunningOrCompleted(t *testing.T) {
    start:=time.Date(2026,11,3,7,30,0,0,time.UTC)
    engine,store,_,_:=smartEngine(t,"codex",start,start.Add(time.Hour),start.Add(5*time.Hour))
    cfg,err:=store.Config()
    if err!=nil{t.Fatal(err)}
    cfg.Tasks[0].Strategy="rolling"
    cfg.Tasks[0].Revision++
    if err=engine.Save(cfg);err!=nil{t.Fatal(err)}
    if err=store.SaveRolling(RollingPlan{TaskID:"smart",AuthID:"a",NextDue:start.UnixMilli(),Source:"upstream"});err!=nil{t.Fatal(err)}
    engine.Tick()
    waitFor(t,func()bool{xs,_:=store.Executions();return len(xs)==1&&xs[0].Status=="waiting"})
    history,err:=engine.History(HistoryQuery{Mode:"task",Page:1,PageSize:20})
    if err!=nil{t.Fatal(err)}
    rows,ok:=history["rows"].([]map[string]any)
    if !ok||len(rows)!=1||rows[0]["status"]!="waiting"{t.Fatalf("waiting status incorrectly aggregated: %+v",history)}
    detail,err:=engine.HistoryDetail(rows[0]["id"].(string))
    if err!=nil{t.Fatal(err)}
    if detail["finished"]!=false{t.Fatalf("waiting run falsely marked complete: %+v",detail)}
}

func TestRollingSnapshotNeverPublishesPastNextDue(t *testing.T) {
    start:=time.Date(2026,11,3,13,0,0,0,time.UTC)
    reset:=start.Add(90*time.Minute)
    laterReset:=start.Add(6*time.Hour+30*time.Minute)
    e,store,host,clock:=smartEngine(t,"codex",start,reset,laterReset)
    cfg,err:=store.Config()
    if err!=nil{t.Fatal(err)}
    cfg.Tasks[0].Strategy="rolling"
    cfg.Tasks[0].Revision++
    if err=e.Save(cfg);err!=nil{t.Fatal(err)}
    get:=func()RollingPlan {
        t.Helper()
        snapshot,err:=e.Snapshot()
        if err!=nil{t.Fatal(err)}
        value,ok:=snapshot["rollingPlans"].(map[string][]RollingPlan)
        if !ok||len(value["smart"])!=1{t.Fatalf("missing rolling account state: %+v",snapshot["rollingPlans"])}
        return value["smart"][0]
    }
    if p:=get();p.Status!="initial"||p.NextDue<=start.UnixMilli(){
        t.Fatalf("first-time bootstrap not represented: %+v",p)
    }
    stale:=start.Add(-time.Hour).UnixMilli()
    if err=store.SaveRolling(RollingPlan{TaskID:"smart",AuthID:"a",NextDue:stale,Source:"upstream"});err!=nil{t.Fatal(err)}
    if p:=get();p.Status!="due"||p.NextDue!=0 {
        t.Fatalf("expired server cursor leaked as future time: %+v",p)
    }
    e.Tick()
    waitFor(t,func()bool{
        items,err:=store.Executions()
        return err==nil&&len(items)==1&&items[0].Status=="waiting"
    })
    xs,err:=store.Executions()
    if err!=nil{t.Fatal(err)}
    waitingUntil:=xs[0].WaitUntil
    // Even if a persisted cursor is stale during the wait, the in-flight
    // execution is the authoritative plan shown to the UI.
    if err=store.SaveRolling(RollingPlan{TaskID:"smart",AuthID:"a",NextDue:stale,Source:"upstream"});err!=nil{t.Fatal(err)}
    if p:=get();p.Status!="waiting"||p.NextDue!=waitingUntil{
        t.Fatalf("waiting execution did not override stale cursor: %+v",p)
    }
    host.mu.Lock()
    host.resetBefore=laterReset.UnixMilli()
    host.mu.Unlock()
    clock.Store(reset.Add(10*time.Second).UnixMilli())
    e.Tick()
    waitFor(t,func()bool{
        items,err:=store.Executions()
        return err==nil&&len(items)==1&&items[0].Status=="skipped"
    })
    if p:=get();p.Status!="scheduled"||p.NextDue!=laterReset.Add(windowSafetyMargin).UnixMilli(){
        t.Fatalf("next rolling time not refreshed after external quota reset: %+v",p)
    }
}

func TestRollingCheckpointAdvancesAfterSkippedQuotaProbe(t *testing.T){
    now:=time.Date(2026,11,3,13,0,0,0,time.UTC)
    reset:=now.Add(4*time.Hour)
    e,store,host,_:=smartEngine(t,"codex",now,reset,reset)
    cfg,err:=store.Config()
    if err!=nil{t.Fatal(err)}
    cfg.Tasks[0].Strategy="rolling"
    cfg.Tasks[0].Revision++
    if err=e.Save(cfg);err!=nil{t.Fatal(err)}
    if err=store.SaveRolling(RollingPlan{TaskID:"smart",AuthID:"a",NextDue:now.Add(-time.Minute).UnixMilli(),Source:"upstream"});err!=nil{t.Fatal(err)}
    if err=store.SaveCheckpoint("a","codex",cfg.Tasks[0].Model,WindowCheckpoint{LastResetAt:reset.UnixMilli(),LastConfirmed:now.Add(-time.Minute).UnixMilli()});err!=nil{t.Fatal(err)}
    host.mu.Lock()
    host.quotaHTTPStatus=503
    host.mu.Unlock()
    e.Tick()
    waitFor(t,func()bool{xs,_:=store.Executions();return len(xs)==1&&xs[0].Status=="skipped"})
    p,ok,err:=store.Rolling("smart","a")
    if err!=nil||!ok||p.NextDue!=reset.Add(windowSafetyMargin).UnixMilli()||p.Source!="checkpoint"{
        t.Fatalf("quota checkpoint was lost after skip: %+v %v %v",p,ok,err)
    }
    host.mu.Lock()
    calls:=host.calls
    host.mu.Unlock()
    if calls!=0{t.Fatalf("quota checkpoint skip dispatched model tokens: %d",calls)}
}
func TestRollingRecentDispatchUsesExplicitEstimatedCursor(t *testing.T){
    now:=time.Date(2026,11,3,13,0,0,0,time.UTC)
    e,store,host,_:=smartEngine(t,"codex",now,now.Add(5*time.Hour),now.Add(10*time.Hour))
    cfg,err:=store.Config()
    if err!=nil{t.Fatal(err)}
    cfg.Tasks[0].Strategy="rolling"
    cfg.Tasks[0].Revision++
    if err=e.Save(cfg);err!=nil{t.Fatal(err)}
    if err=store.SaveRolling(RollingPlan{TaskID:"smart",AuthID:"a",NextDue:now.Add(-time.Minute).UnixMilli(),Source:"upstream"});err!=nil{t.Fatal(err)}
    lastAttempt:=now.Add(-30*time.Second)
    if err=store.SaveCheckpoint("a","codex",cfg.Tasks[0].Model,WindowCheckpoint{LastAttempt:lastAttempt.UnixMilli()});err!=nil{t.Fatal(err)}
    host.mu.Lock()
    host.quotaHTTPStatus=503
    host.mu.Unlock()
    e.Tick()
    waitFor(t,func()bool{xs,_:=store.Executions();return len(xs)==1&&xs[0].Status=="skipped"})
    p,ok,err:=store.Rolling("smart","a")
    if err!=nil||!ok||p.NextDue!=lastAttempt.Add(5*time.Hour+windowSafetyMargin).UnixMilli()||p.Source!="estimated"{
        t.Fatalf("recent dispatch estimate not persisted: %+v %v %v",p,ok,err)
    }
    host.mu.Lock()
    calls:=host.calls
    host.mu.Unlock()
    if calls!=0{t.Fatalf("dedupe guard dispatched duplicate model tokens: %d",calls)}
}

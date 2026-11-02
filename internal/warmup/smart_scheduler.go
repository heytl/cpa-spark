package warmup

import (
    "database/sql"
    "encoding/json"
    "strings"
    "time"
)

type WindowCheckpoint struct {
    LastAttempt int64 `json:"lastAttempt"`
    LastResetAt int64 `json:"lastResetAt,omitempty"`
    LastConfirmed int64 `json:"lastConfirmed,omitempty"`
}
type RollingPlan struct {
    TaskID string `json:"taskId"`
    AuthID string `json:"authId"`
    NextDue int64 `json:"nextDue"`
    Source string `json:"source"`
    LastExecution string `json:"lastExecution"`
}
func normalizedStrategy(t Task) string {
    if t.Strategy == "rolling" { return "rolling" }
    return "fixed"
}
func windowKey(provider, model string) string {
    if provider=="codex" { return "codex:5h" }
    family:=antigravityFamily(model)
    if family!="" { return "antigravity:"+family+":5h" }
    return provider+":"+strings.ToLower(model)
}
func checkpointKey(authID, provider, model string) string {
    return authID+"\x00"+windowKey(provider,model)
}
func rollingKey(taskID,authID string) string {return taskID+"\x00"+authID}

func (s *Store) Checkpoint(authID,provider,model string) (WindowCheckpoint,error) {
    var raw []byte
    err:=s.db.QueryRow("SELECT body FROM window_checkpoints WHERE id=?",checkpointKey(authID,provider,model)).Scan(&raw)
    if err==sql.ErrNoRows {return WindowCheckpoint{},nil}
    if err!=nil {return WindowCheckpoint{},err}
    var value WindowCheckpoint
    err=json.Unmarshal(raw,&value)
    return value,err
}
func (s *Store) SaveCheckpoint(authID,provider,model string,value WindowCheckpoint) error {
    _,err:=s.db.Exec("INSERT INTO window_checkpoints(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body",checkpointKey(authID,provider,model),marshal(value))
    return err
}
func (s *Store) Rolling(taskID,authID string) (RollingPlan,bool,error) {
    var raw []byte
    err:=s.db.QueryRow("SELECT body FROM rolling_plans WHERE id=?",rollingKey(taskID,authID)).Scan(&raw)
    if err==sql.ErrNoRows {return RollingPlan{},false,nil}
    if err!=nil {return RollingPlan{},false,err}
    var p RollingPlan
    err=json.Unmarshal(raw,&p)
    return p,true,err
}
func (s *Store) SaveRolling(p RollingPlan) error {
    _,err:=s.db.Exec("INSERT INTO rolling_plans(id,body) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET body=excluded.body",rollingKey(p.TaskID,p.AuthID),marshal(p))
    return err
}
func (s *Store) DeleteRolling(taskID,authID string) error {
    _,err:=s.db.Exec("DELETE FROM rolling_plans WHERE id=?",rollingKey(taskID,authID))
    return err
}
// Due rolling triggers use a persistent per-task/account cursor, not local
// ticker state. They survive plugin and CPA process restarts.
func (e *Engine) enqueueRolling(c Config,now time.Time) error {
    if c.Settings.Paused { return nil }
    for _,t:=range c.Tasks {
        if !t.Enabled || normalizedStrategy(t)!="rolling" {continue}
        for _,authID:=range Targets(t,c.Groups) {
            if c.Overrides[authID] {continue}
            p,ok,err:=e.store.Rolling(t.ID,authID)
            if err!=nil {return err}
            if !ok || p.NextDue<=0 || now.UnixMilli()<p.NextDue {continue}
            isolated:=t
            isolated.GroupIDs=nil
            isolated.AccountIDs=[]string{authID}
            isolated.Excludes=nil
            at:=time.UnixMilli(p.NextDue)
            if _,err=e.enqueue([]Task{isolated},c,at,"rolling",at.UTC().Format(time.RFC3339Nano));err!=nil {return err}
        }
    }
    return nil
}
func (e *Engine) saveNextRolling(x Execution, observed WindowObservation) error {
    if normalizedStrategy(x.Task)!="rolling" {return nil}
    current,err:=e.store.Config()
    if err!=nil {return err}
    for _,o:=range x.Origins {
        if !o.Active {continue}
        // A completed request must not re-create a cursor after the user
        // disabled, deleted or edited the task while it was in flight.
        stillEnabled:=false
        for _,t:=range current.Tasks {
            if t.ID==o.TaskID && t.Enabled && normalizedStrategy(t)=="rolling" && t.Revision==x.Task.Revision {
                stillEnabled=true
                break
            }
        }
        if !stillEnabled {continue}
        next:=observed.ResetAt
        source:="upstream"
        if next<=e.now().UnixMilli() {
            // No upper-bound or model group information is available. Do not
            // pretend to have a verified Antigravity cycle.
            if x.Provider!="codex" {continue}
            next=e.now().Add(5*time.Hour).UnixMilli()
            source="estimated"
        }
        return e.store.SaveRolling(RollingPlan{
            TaskID:o.TaskID,AuthID:x.AuthID,
            NextDue:next+windowSafetyMargin.Milliseconds(),Source:source,LastExecution:x.ID,
        })
    }
    return nil
}

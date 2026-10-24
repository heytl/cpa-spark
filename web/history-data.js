// Actual account executions are unique even when several task runs share them.
const CPAHistory = (() => {
 function accounts(runs, accounts){
  const labels=new Map(accounts.map(a=>[a.id,a])), rows=new Map();
  for(const run of runs)for(const [index,item] of (run.items||[]).entries()){
   const id=item.id||run.id+':'+item.authId+':'+index;
   if(!rows.has(id))rows.set(id,{id,authId:item.authId,account:labels.get(item.authId),model:run.model,mode:run.mode,status:item.executionStatus||item.status,started:Number(item.started||run.started),finished:Number(item.finished||0),runId:run.id,runs:[]});
   const row=rows.get(id);if(!row.runs.some(r=>r.id===run.id))row.runs.push(run);
  }
  return [...rows.values()].sort((a,b)=>b.started-a.started||a.id.localeCompare(b.id));
 }
 return Object.freeze({accounts});
})();

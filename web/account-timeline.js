// Both sides of the account timeline use rolling 24-hour windows.
function accountTimelineData(id,now=Date.now()){
 const day=24*60*60*1000,tasks=state.tasks.filter(t=>targetIDs(t).includes(id)),map=new Map();
 for(const task of tasks.filter(t=>t.enabled))for(const at of occurrences(task,Math.max(24,task.times.length*2),now)){
  if(at>now+day)continue;const key=at+profile(task);if(!map.has(key))map.set(key,{at,tasks:[],model:task.model});map.get(key).tasks.push(task);
 }
 const past=new Map();
 for(const run of (typeof accountTimelineRuns!=='undefined'&&accountTimelineAccount===id&&accountTimelineRuns?accountTimelineRuns:state.runs))for(const item of run.items){
  if(item.authId!==id||['queued','running'].includes(item.status))continue;
  const at=Number(item.finished||item.started||run.started);if(!(at>now-day&&at<=now))continue;
  const key=item.id||run.id+':'+id;if(past.has(key))continue;
  past.set(key,{at,runId:run.id,name:run.name,model:run.model,status:item.status,message:item.message});
 }
 return {tasks,upcoming:[...map.values()].sort((a,b)=>a.at-b.at),past:[...past.values()].sort((a,b)=>b.at-a.at)};
}

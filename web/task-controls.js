// Pure task-editor rules. Model IDs and reasoning capabilities come from CPA.
const CPATaskControls = (() => {
 const ranks=['none','minimal','low','medium','high','xhigh','max','auto'];
 function modelOptions(catalogs){
  if(!catalogs.length)return [];
  const models=new Map();for(const list of catalogs)for(const m of list){if(typeof m.id!=='string'||/image|embedding|audio|transcrib|tts/i.test(m.id))continue;models.set(m.id,{...models.get(m.id),...m})}
  return [...models.values()].sort((a,b)=>a.id.localeCompare(b.id,undefined,{numeric:true}));
 }
 function preferredModel(models,provider){
  const ranked=[...models].sort((a,b)=>{
   const rank=id=>provider==='codex'?(/luna/i.test(id)?0:1):(/gemini.*flash.*lite/i.test(id)?0:/gemini.*flash/i.test(id)?1:2);
   return rank(a.id)-rank(b.id)||b.id.localeCompare(a.id,undefined,{numeric:true});
  });return ranked[0]?.id||'';
 }
 function efforts(model){
  const thinking=model?.thinking;
  if(!thinking)return [{value:'',label:'模型默认（CPA 未提供推理能力信息）'}];
  const levels=Array.isArray(thinking.levels)?[...new Set(thinking.levels.map(s=>String(s).toLowerCase()).filter(s=>ranks.includes(s)))]:[];
  if(thinking.zero_allowed&&!levels.includes('none')&&levels.length)levels.push('none');
  if(levels.length)return levels.sort((a,b)=>ranks.indexOf(a)-ranks.indexOf(b)).map(value=>({value,label:({none:'关闭推理 · 最省 token',minimal:'最低',low:'低',medium:'中',high:'高',xhigh:'更高',max:'最高',auto:'自动'})[value]}));
  const min=Number(thinking.min||0),max=Number(thinking.max||0);
  const budgets=[...(thinking.zero_allowed?[0]:[]),...(min>0?[min]:[])];
  if(budgets.length&&max>0&&max!==min)budgets.push(max);
  const choices=[...new Set(budgets)].sort((a,b)=>a-b).map(n=>({value:'budget:'+n,label:n===0?'关闭推理 · 0 token':`${n} tokens${n===min?' · 最低预算':''}`}));
  if(thinking.dynamic_allowed)choices.push({value:'auto',label:'自动预算'});
  return choices.length?choices:[{value:'',label:'模型默认（无需推理配置）'}];
 }
 function nextTime(times){
  let minutes=times.length?Number(times.at(-1).slice(0,2))*60+Number(times.at(-1).slice(3))+60:6*60;
  for(let i=0;i<1440;i++,minutes++){
   const n=minutes%1440,value=String(Math.floor(n/60)).padStart(2,'0')+':'+String(n%60).padStart(2,'0');
   if(!times.includes(value))return value;
  }return '06:00';
 }
 return Object.freeze({modelOptions,preferredModel,efforts,nextTime});
})();

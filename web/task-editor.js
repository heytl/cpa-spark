// Production task controls: account-specific CPA catalogs, searchable zones and top-layer menus.
let taskCatalogOwner='',taskCatalogGeneration=0;
const taskCatalog=new Map(),taskDefinitions=new Map();
function catalogOwner(){if(taskCatalogOwner!==managementKey){taskCatalogOwner=managementKey;taskCatalogGeneration++;taskCatalog.clear();taskDefinitions.clear()}}
function catalogTargets(t){return targetIDs(t).map(account).filter(a=>a&&a.provider===t.provider)}
function modelContext(t){
 catalogOwner();const accounts=catalogTargets(t),entries=accounts.map(a=>taskCatalog.get(a.id));
 if(!accounts.length)return {models:[],message:'请选择至少一个仍存在于 CPA 的同提供商账号；暂停或暂时不可用不影响读取模型目录。',state:'no-targets',ready:false};
 if(entries.some(e=>!e||e.loading)||!taskDefinitions.has(t.provider)||taskDefinitions.get(t.provider)?.loading)return {models:[],message:'正在从 CPA 获取目标账号的模型与推理能力…',state:'loading',ready:false};
 if(entries.every(e=>e.error))return {models:[],message:'获取账号模型失败，请点击刷新模型重试',state:'failed',ready:false};
 const definitions=taskDefinitions.get(t.provider)?.models||[];
 const models=CPATaskControls.modelOptions(entries.filter(e=>!e.error).map(e=>e.models||[])).map(m=>({...definitions.find(d=>d.id===m.id),...m}));
 return {models,ready:true,state:'ready',message:models.length?'显示任一所选账号支持的模型；禁用或过期账号在执行时跳过，模型权限由 CPA 校验。':'未获取到所选账号支持的文本模型，请刷新或调整账号。'};
}
function taskModelReady(t){const c=modelContext(t),model=c.models.find(m=>m.id===t.model);return c.ready&&!!model&&CPATaskControls.efforts(model).some(e=>e.value===(t.effort||''))}
async function loadTaskCatalog(t){
 catalogOwner();const generation=taskCatalogGeneration,provider=t.provider,jobs=[];
 for(const a of catalogTargets(t))if(!taskCatalog.has(a.id)){
  taskCatalog.set(a.id,{loading:true});jobs.push(async()=>{
   try{const response=await cpaAPI('auth-files/models?name='+encodeURIComponent(a.id));if(generation!==taskCatalogGeneration)return;if(!Array.isArray(response.models))throw Error('invalid model catalog');taskCatalog.set(a.id,{models:response.models.filter(m=>typeof m.id==='string')})}
   catch{if(generation===taskCatalogGeneration)taskCatalog.set(a.id,{error:true})}
  });
 }
 if(!taskDefinitions.has(provider)){
  taskDefinitions.set(provider,{loading:true});jobs.push(async()=>{
   try{const response=await cpaAPI('model-definitions/'+encodeURIComponent(provider));if(generation===taskCatalogGeneration)taskDefinitions.set(provider,{models:Array.isArray(response.models)?response.models:[]})}
   catch{if(generation===taskCatalogGeneration)taskDefinitions.set(provider,{models:[]})}
  });
 }
 if(!jobs.length)return;
 let index=0;await Promise.all(Array.from({length:Math.min(4,jobs.length)},async()=>{while(index<jobs.length)await jobs[index++]()}));
 if(generation===taskCatalogGeneration&&draft?.provider===provider&&$('#taskDialog').open&&taskStep===2){readTaskForm();renderTaskDialog()}
}
function chooseTaskDefaults(t){
 const context=modelContext(t);if(!context.ready)return context;
 const editing=!!task(t.id);
 if(!editing&&!context.models.some(m=>m.id===t.model))t.model=CPATaskControls.preferredModel(context.models,t.provider);
 const model=context.models.find(m=>m.id===t.model),choices=CPATaskControls.efforts(model);
 if(!editing&&(!t._modelChosen||!choices.some(c=>c.value===(t.effort||'')))){t.effort=choices[0].value;t._modelChosen=t.model}
 return context;
}
const zoneAliases={'Asia/Shanghai':'上海 北京','Asia/Hong_Kong':'香港','Asia/Taipei':'台北','Asia/Tokyo':'东京','Asia/Singapore':'新加坡','Europe/London':'伦敦','America/New_York':'纽约','America/Los_Angeles':'洛杉矶','UTC':'协调世界时'};
const taskZones=[...new Set(['Asia/Shanghai','UTC',...CPA_TIME_ZONES,...(Intl.supportedValuesOf?.('timeZone')||[])])];
zonesOptions=function(value,query=''){
 const needle=query.trim().toLowerCase();const zones=taskZones.filter(z=>z===value||!needle||(z+' '+(zoneAliases[z]||'')).replaceAll('_',' ').toLowerCase().includes(needle));
 if(value&&!zones.includes(value))zones.unshift(value);
 return zones.map(z=>`<option value="${esc(z)}" ${z===value?'selected':''}>${esc(z)}${zoneAliases[z]?' · '+esc(zoneAliases[z]):''}</option>`).join('');
};
const editorBase=renderTaskDialog;
renderTaskDialog=function(){
 if(!draft)return;
 let context;if(taskStep===2){void loadTaskCatalog(draft);context=chooseTaskDefaults(draft)}
 editorBase();const dialog=$('#taskDialog');
 if(taskStep===1){
  const list=dialog.querySelector('.time-list');
  list.innerHTML=draft.times.map((v,i)=>{const [hour,minute]=v.split(':');return `<div class="time-entry"><span class="time-index">${i+1}</span><label><span class="sr-only">触发时间 ${i+1} 小时</span><select class="field clock-select" data-clock-hour="${i}">${Array.from({length:24},(_,n)=>String(n).padStart(2,'0')).map(h=>`<option ${h===hour?'selected':''}>${h}</option>`).join('')}</select></label><span class="clock-colon">:</span><label><span class="sr-only">触发时间 ${i+1} 分钟</span><select class="field clock-select" data-clock-minute="${i}">${Array.from({length:60},(_,n)=>String(n).padStart(2,'0')).map(m=>`<option ${m===minute?'selected':''}>${m}</option>`).join('')}</select></label><input type="hidden" data-time-index="${i}" value="${esc(v)}"><button class="iconbtn" data-action="remove-time" data-id="${i}" aria-label="删除时间 ${esc(v)}">${icon('x')}</button></div>`}).join('')+`<button class="add-time" data-action="add-time" data-i18n="message.17101b8ca4ca">+ 添加时间</button>`;
  list.insertAdjacentHTML('afterend','<div class="time-presets"><span data-i18n="message.5fe7555f9205">快捷设置</span><button class="btn compact" data-editor-preset="06:00,11:00,16:00">06:00 / 11:00 / 16:00</button><button class="btn compact" data-editor-preset="00:00,05:00,10:00,15:00,20:00" data-i18n="message.5944a0eaeb33">当天间隔 5 小时</button><button class="btn compact" data-editor-sort data-i18n="message.471536ca38de">排序并去重</button></div>');
  $('#taskZone').insertAdjacentHTML('beforebegin','<input class="field zone-search" data-zone-search="taskZone" type="search" aria-label="搜索任务时区" placeholder="搜索时区，例如 上海、Tokyo、America…">');
 }
 if(taskStep===2){
  const current=context.models.find(m=>m.id===draft.model),choices=CPATaskControls.efforts(current),unavailable=draft.model&&!current;
  const control=$('#taskModel');control.outerHTML=`<select class="field" id="taskModel" ${!context.ready||!context.models.length?'disabled':''}><option value="">${context.state==='loading'?'正在获取模型…':context.state==='failed'?'获取模型失败':context.state==='no-targets'?'请先选择目标账号':context.models.length?'请选择模型':'没有可用模型'}</option>${context.models.map(m=>`<option value="${esc(m.id)}" ${m.id===draft.model?'selected':''}>${esc(m.id)}</option>`).join('')}${unavailable?`<option value="${esc(draft.model)}" selected disabled>${esc(draft.model)} · 当前不可用</option>`:''}</select>`;
  const hint=$('#taskModel').parentElement.querySelector('.field-hint');hint.textContent=context.message;
  hint.insertAdjacentHTML('afterend','<button type="button" class="btn compact catalog-refresh" data-refresh-models data-i18n="message.a76952e53abf">刷新模型</button>');
  const oldEffort=$('#taskEffort')?.closest('.field-group');if(oldEffort)oldEffort.remove();
  const preserved=!!task(draft.id)&&!choices.some(c=>c.value===(draft.effort||''));
  $('#taskModel').closest('.field-group').insertAdjacentHTML('afterend',`<div class="field-group"><label class="label" for="taskEffort" data-i18n="message.c8c14507b2d3">推理强度</label><select class="field" id="taskEffort" ${!current?'disabled':''}>${choices.map(c=>`<option value="${esc(c.value)}" ${c.value===(draft.effort||'')?'selected':''}>${esc(c.label)}</option>`).join('')}${preserved?`<option value="${esc(draft.effort||'')}" selected disabled>${esc(draft.effort||'模型默认')} · 请重新选择</option>`:''}</select><div class="field-hint">${current?.thinking?'从 CPA 模型能力自动获取；新任务默认选择最低可用强度。':'CPA 未提供推理能力时，保留模型默认，不猜测支持的参数。'}</div></div>`);
  syncTaskPreviewButton();void requestTaskPreview();
 }
};
const validateTaskBase=validateDraft;
validateDraft=function(step=2){const error=validateTaskBase(step);if(error)return error;if(step>=1&&!taskZones.includes(draft.timezone))return '请选择有效的 IANA 时区';if(step>=2&&!taskModelReady(draft))return '请等待模型获取完成，并选择目标账号支持的模型和推理强度';return ''};
document.addEventListener('input',event=>{
 const selectId=event.target.dataset.zoneSearch;if(selectId){const select=document.getElementById(selectId);select.innerHTML=zonesOptions(select.value,event.target.value)}
});
document.addEventListener('change',event=>{
 const el=event.target;if(!draft)return;
 if(el.dataset.clockHour!==undefined||el.dataset.clockMinute!==undefined){const index=el.dataset.clockHour??el.dataset.clockMinute;const h=document.querySelector(`[data-clock-hour="${index}"]`).value,m=document.querySelector(`[data-clock-minute="${index}"]`).value;document.querySelector(`[data-time-index="${index}"]`).value=h+':'+m;readTaskForm();renderTaskDialog()}
 if(el.id==='taskProvider')delete draft._modelChosen;
 if(el.id==='taskModel'){readTaskForm();draft.model=el.value;draft._modelChosen=el.value;draft.effort=CPATaskControls.efforts(modelContext(draft).models.find(m=>m.id===el.value))[0].value;renderTaskDialog();event.stopImmediatePropagation()}
},true);
document.addEventListener('click',event=>{
 const el=event.target.closest('[data-editor-preset],[data-editor-sort],[data-refresh-models]');if(!el)return;
 event.preventDefault();readTaskForm();
 if(el.dataset.editorPreset!==undefined)draft.times=el.dataset.editorPreset.split(',');
 if(el.hasAttribute('data-editor-sort'))draft.times=[...new Set(draft.times)].sort();
 if(el.hasAttribute('data-refresh-models')){taskCatalogGeneration++;taskCatalog.clear();taskDefinitions.clear();invalidateTaskPreview()}
 renderTaskDialog();
});
// Move the menu out of the scrolling table so every action remains reachable.
const renderWorkspaceBase=render;
function positionTaskMenu(){
 const menu=document.querySelector('.row-menu');if(!menu)return;
 const trigger=[...document.querySelectorAll('[data-action="menu"]')].find(el=>el.dataset.id===openMenu);if(!trigger){menu.remove();return}
 if(menu.parentElement!==document.body)document.body.append(menu);
 const anchor=trigger.getBoundingClientRect(),height=menu.getBoundingClientRect().height,width=menu.getBoundingClientRect().width;
 menu.style.left=Math.max(8,Math.min(innerWidth-width-8,anchor.right-width))+'px';menu.style.top=(anchor.bottom+height+8<innerHeight?anchor.bottom+6:Math.max(8,anchor.top-height-6))+'px';
}
render=function(...args){document.querySelectorAll('body > .row-menu').forEach(m=>m.remove());renderWorkspaceBase(...args);positionTaskMenu()};
window.addEventListener('resize',positionTaskMenu);
document.addEventListener('scroll',positionTaskMenu,true);

document.addEventListener('click',event=>{const menu=document.querySelector('body > .row-menu');if(menu&&!event.target.closest('.row-menu,[data-action="menu"]')){openMenu=null;menu.remove()}},true);

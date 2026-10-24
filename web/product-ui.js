// Product UI: live server owns all mutations and execution results.
// Shared presentation and live interaction controls.
const previewRenderBase=render;
function simplifyPreview(){
 document.querySelectorAll('.tip-panel,.aside-foot,.bottom-note,.stat-foot').forEach(el=>el.remove());
 document.querySelectorAll('.drawer-header p').forEach(el=>el.remove());
 const redundant=['每个任务使用同一提供商，模型配置更清晰。','也可以点击星期，组合自定义运行日期。','分组与账号取并集。同一个账号即使多次选中，也只计算一次。'];
 document.querySelectorAll('.field-hint,.notice').forEach(el=>{if(redundant.includes(el.textContent.trim()))el.remove()});
}
render=function(...args){previewRenderBase(...args);simplifyPreview()};
const previewTaskRenderBase=renderTaskDialog;
renderTaskDialog=function(){previewTaskRenderBase();simplifyPreview();const modelHint=document.querySelector('#taskModel')?.parentElement.querySelector('.field-hint');if(modelHint&&modelContext(draft).ready)modelHint.textContent='任一所选账号支持的模型';const effortHint=document.querySelector('#taskEffort')?.parentElement.querySelector('.field-hint');if(effortHint&&modelContext(draft).ready)effortHint.textContent='默认选择最低可用强度';document.querySelectorAll('#taskDialog .notice').forEach(el=>{if(el.textContent.includes('发送一条短文本请求'))el.innerHTML=icon('bolt')+'发送简短 OK 请求，使用正常账号额度。';if(el.textContent.includes('保存后从下一个未来时间点'))el.innerHTML=icon('clock')+'从下次计划开始，不补发错过的请求。'})};
heading=function(title,desc,actions=''){return `<div class="page-heading"><div><h1>${title}</h1></div><div class="heading-actions">${actions}</div></div>`};
renderSettings=function(){const p=settingsDraft||state.settings;return heading('设置','')+`<div class="settings-grid"><section class="panel settings-card"><h2 data-i18n="message.173a9d27e121">调度偏好</h2><div class="settings-row"><label for="settingZone" data-i18n="message.824f34cb768d">默认时区</label><select id="settingZone" class="field">${zonesOptions(p.timezone)}</select></div><div class="settings-row"><label for="settingConcurrency" data-i18n="message.c69f8d055729">并发账号</label><input id="settingConcurrency" type="range" min="1" max="8" value="${p.concurrency}"><span id="concurrencyValue">${p.concurrency}</span></div><div class="settings-row"><label for="settingLate" data-i18n="message.de77bd7b8ff0">允许迟到</label><select id="settingLate" class="field">${[60,120,300].map(v=>`<option value="${v}" ${v===p.late?'selected':''}>${v/60} 分钟</option>`).join('')}</select></div><div class="settings-row"><label for="settingRetention" data-i18n="message.d0d87a818bdc">保留记录</label><select id="settingRetention" class="field">${[7,30,90].map(v=>`<option value="${v}" ${v===p.retention?'selected':''}>${v} 天</option>`).join('')}</select></div><button class="btn primary" data-action="save-settings" data-i18n="message.c8550237ba70">保存设置</button></section><section class="panel settings-card"><h2 data-i18n="message.038604774032">语言与主题</h2><p class="small muted" style="margin-top:12px" data-i18n="message.df1c65acff2a">跟随 CPA 后台</p></section><section class="panel settings-card brand-card"><div class="brand-lockup">${renderBrandIdentity()}</div><div class="brand-tagline">轻量预热，按时就绪。</div><p class="brand-intro" data-i18n="message.767519cb5d6f">为 CLIProxyAPI 提供轻量的账号定时预热、分组调度与额度窗口可视化。</p><div class="brand-features"><span data-i18n="message.2a058ae75898">独立任务</span><span data-i18n="message.88f6f0c57e42">分组调度</span><span data-i18n="message.0a394cb2dbea">执行追踪</span></div><div class="brand-store-title">智能账号预热与额度窗口调度</div></section></div>`};
render();
const centeredRunDetailBase=renderRunDetail;
renderRunDetail=function(run){document.getElementById('detailDialog').classList.add('execution-detail');centeredRunDetailBase(run)};
const originalAccountDetail=renderAccountDetail;
renderAccountDetail=function(){document.getElementById('accountDialog').classList.remove('execution-detail');originalAccountDetail()};

// Static review: account enabled state is independent of warmup overrides and login health.
function previewAccountEnabled(a){return !!a&&!a.disabled&&a.status!=='disabled'}
function previewEnabledFirst(a,b){return Number(!previewAccountEnabled(a))-Number(!previewAccountEnabled(b))}
function previewStatusFilter(){return `<select class="field" aria-label="账号状态筛选" data-filter="accountStatus">${[['all','全部状态'],['disabled','已禁用'],['enabled','已启用']].map(([value,label])=>`<option value="${value}" ${(filters.accountStatus||'all')===value?'selected':''}>${label}</option>`).join('')}</select>`}
let quickWarmup=null,quickWarmupSequence=0;
const quickWarmupDialog=document.createElement('dialog');quickWarmupDialog.id='quickWarmupDialog';quickWarmupDialog.className='modal quick-warmup';quickWarmupDialog.setAttribute('aria-labelledby','quickWarmupHeading');document.body.append(quickWarmupDialog);
quickWarmupDialog.addEventListener('close',()=>{quickWarmupSequence++;quickWarmup=null});
function quickEligible(a){return previewAccountEnabled(a)&&!skipReason(a)}
function renderQuickWarmup(){
 const q=quickWarmup;if(!q)return;
 const all=q.ids.map(account).filter(Boolean),eligible=all.filter(quickEligible),skipped=all.filter(a=>!quickEligible(a));
 const context=q.context,model=context?.models.find(m=>m.id===q.model),choices=CPATaskControls.efforts(model),ready=!!model&&eligible.length>0;
 quickWarmupDialog.innerHTML=`<div class="modal-head"><div><h2 id="quickWarmupHeading" data-i18n="message.f03bb06e75ce">立即预热</h2><p class="small muted quick-target-name">${esc(q.name)}</p></div><button class="iconbtn" data-quick-close aria-label="关闭立即预热">${icon('x')}</button></div><div class="modal-body"><div class="quick-summary"><div><strong>${all.length}</strong><span data-i18n="message.4c74e040c922">目标账号</span></div><div><strong>${eligible.length}</strong><span data-i18n="message.7ab7815d503d">可执行</span></div><div><strong>${skipped.length}</strong><span data-i18n="message.fc50a99caae0">跳过</span></div></div><div class="field-group"><label class="label" for="quickModel" data-i18n="message.c98e118e0a43">模型</label><select class="field" id="quickModel" ${!context?.models.length?'disabled':''}>${context?.models.length?context.models.map(m=>`<option value="${esc(m.id)}" ${m.id===q.model?'selected':''}>${esc(m.id)}</option>`).join(''):`<option>${q.loading?'正在获取模型…':eligible.length?'没有可用模型':'没有可执行账号'}</option>`}</select></div><div class="field-group"><label class="label" for="quickEffort" data-i18n="message.c8c14507b2d3">推理强度</label><select class="field" id="quickEffort" ${!model?'disabled':''}>${choices.map(e=>`<option value="${esc(e.value)}" ${e.value===q.effort?'selected':''}>${esc(e.label)}</option>`).join('')}</select></div>${skipped.length?`<div class="notice neutral">${icon('info')}跳过 ${skipped.length} 个账号：${skipped.map(a=>esc(a.name)+'（'+esc(skipReason(a)||'已禁用')+'）').join('、')}</div>`:''}${q.error?`<p class="form-error" role="alert">${esc(q.error)}</p>`:''}<p class="small muted quick-note" data-i18n="message.24015650cad5">仅执行一次，不新增定时任务。请求使用正常账号额度。</p></div><div class="modal-foot"><button class="btn" data-quick-close data-i18n="message.2cd0f3be8738">取消</button><button class="btn primary" data-quick-confirm ${!ready||q.loading?'disabled':''}>${icon('bolt')}立即预热</button></div>`;
}
async function openQuickWarmup(kind,id){
 const g=kind==='group'?group(id):null,t=kind==='task'?task(id):null,a=kind==='account'?account(id):null;
 if(!g&&!t&&!a)return;
 const ids=[...new Set(g?g.members:t?targetIDs(t):[a.id])],provider=(g||t||a).provider;
 const seq=++quickWarmupSequence;
 quickWarmup={name:(g||t||a).name,taskId:t?.id,ids,provider,model:'',effort:'',loading:true};renderQuickWarmup();if(!quickWarmupDialog.open)quickWarmupDialog.showModal();
 const eligible=ids.map(account).filter(a=>a&&a.provider===provider&&quickEligible(a));
 if(!eligible.length){quickWarmup.loading=false;renderQuickWarmup();return}
 const requestTask={id:'quick-review',name:quickWarmup.name,provider,accountIds:eligible.map(a=>a.id),groupIds:[],excludes:[]};
 await loadTaskCatalog(requestTask);
 if(seq!==quickWarmupSequence||!quickWarmup||!quickWarmupDialog.open)return;
 const context=modelContext(requestTask);quickWarmup.context=context;quickWarmup.loading=false;
 quickWarmup.model=t&&context.models.some(m=>m.id===t.model)?t.model:CPATaskControls.preferredModel(context.models,provider);
 const choices=CPATaskControls.efforts(context.models.find(m=>m.id===quickWarmup.model));quickWarmup.effort=t&&choices.some(e=>e.value===t.effort)?t.effort:choices[0].value;
 if(!context.ready)quickWarmup.error=context.message;
 renderQuickWarmup();
}
async function confirmQuickWarmup(button){
 const q=quickWarmup;if(!q||button.disabled||mutating||!connected)return;
 const eligible=q.ids.map(account).filter(a=>a&&a.provider===q.provider&&quickEligible(a));if(!eligible.length){toast('没有可执行账号','info');return}
 button.disabled=true;mutating=true;const pending=toast('正在提交预热…','loader');
 try{const result=await api(q.taskId?'run':'run-once',{taskId:q.taskId,key:crypto.randomUUID(),task:{name:'立即预热 · '+q.name,provider:q.provider,model:q.model,effort:q.effort,accountIds:q.ids,groupIds:[],excludes:[],timezone:state.settings.timezone}});quickWarmupDialog.close();applySnapshot(await api('state'));showRun(result.runId);pending.remove();toast(t('warmup.submitted',{count:eligible.length}),'info')}
 catch(error){pending.remove();toast('预热提交失败：'+error.message,'alert');if(quickWarmupDialog.open){q.error=error.message;renderQuickWarmup()}}
 finally{mutating=false;if(button.isConnected)button.disabled=false}
}
document.addEventListener('click',event=>{
 const el=event.target.closest('[data-quick-warmup],[data-quick-confirm],[data-quick-close]');if(!el)return;
 event.preventDefault();event.stopImmediatePropagation();
 if(el.hasAttribute('data-quick-close')){quickWarmupDialog.close();return}
 if(el.hasAttribute('data-quick-confirm')){confirmQuickWarmup(el);return}
 void openQuickWarmup(el.dataset.quickWarmup,el.dataset.id);
},true);
quickWarmupDialog.addEventListener('change',event=>{
 const q=quickWarmup;if(!q)return;
 if(event.target.id==='quickModel'){q.model=event.target.value;q.effort=CPATaskControls.efforts(q.context.models.find(m=>m.id===q.model))[0].value;renderQuickWarmup()}
 if(event.target.id==='quickEffort')q.effort=event.target.value;
});
openRunPreview=function(id){void openQuickWarmup('task',id)};
render();

// Shared controls for the self-contained design preview.
function previewZoneLabel(zone){
 const offset=new Intl.DateTimeFormat('en',{timeZone:zone,timeZoneName:'longOffset'}).formatToParts(Date.now()).find(p=>p.type==='timeZoneName').value;
 const raw=offset.replace('GMT','');
 const match=raw.match(/^([+-])(\d{2}):(\d{2})$/);
 const utc=!raw?'UTC+0':match?'UTC'+match[1]+Number(match[2])+(match[3]==='00'?'':':'+match[3]):'UTC'+raw;
 return zone+' · '+utc;
}
zonesOptions=function(value){const zones=[...new Set(['Asia/Shanghai','UTC',value,...taskZones,...(Intl.supportedValuesOf?Intl.supportedValuesOf('timeZone'):[])])].filter(Boolean);return zones.map(z=>`<option value="${esc(z)}" ${z===value?'selected':''}>${esc(previewZoneLabel(z))}</option>`).join('')};
function previewTimestamp(ms,zone=state.settings.timezone){const p=new Intl.DateTimeFormat('en-GB',{timeZone:zone,year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:'h23'}).formatToParts(ms);const v=k=>p.find(x=>x.type===k).value;return `${v('year')}-${v('month')}-${v('day')} ${v('hour')}:${v('minute')}:${v('second')}`}
function previewDefaultName(base,items){base=uiText(base);const names=new Set(items.map(x=>x.name));let n=1;while(names.has(base+' '+n))n++;return base+' '+n}
const namedTaskBase=openTask;openTask=function(id=null,gid=null){namedTaskBase(id,gid);if(!id){draft.name=previewDefaultName('预热任务',state.tasks);document.getElementById('taskName').value=draft.name}};
const namedGroupBase=openGroup;openGroup=function(id=null){namedGroupBase(id);if(!id){groupDraft.name=previewDefaultName('账号分组',state.groups);document.getElementById('groupName').value=groupDraft.name}};
function previewHistoryMatches(r){return (filters.historyStatus==='all'||runStatus(r)===filters.historyStatus)&&(!filters.historyQuery||[r.name,r.model,r.id].join(' ').toLowerCase().includes(filters.historyQuery.toLowerCase()))&&(!filters.historyDate||dateKey(r.started)===filters.historyDate)}
const clearHistoryBase=renderHistory;
renderHistory=function(){
 const count=state.runs.filter(r=>r.finished&&previewHistoryMatches(r)).length,all=state.runs.filter(r=>r.finished).length;
 return clearHistoryBase().replace('<button class="btn" data-action="export-history">',`<button class="btn" data-clear-history="filtered" ${!count?'disabled':''}>${icon('trash')}删除筛选结果 <span class="badge">${count}</span></button><button class="btn danger" data-clear-history="all" ${!all?'disabled':''} data-i18n="message.55f1033fab69">删除全部</button><button class="btn" data-action="export-history">`).replace('<section class="panel">','<p class="history-clear-note" data-i18n="message.d05224a246b1">删除筛选结果：按当前日期、关键词和执行结果筛选。删除全部：忽略筛选条件。执行中的记录会保留。</p><section class="panel">').replace(/(<button class="iconbtn" data-action="run-detail" data-id="([^"]+)"[^>]*>[\s\S]*?<\/button>)/g,(_,button,id)=>{const r=state.runs.find(r=>r.id===id);return button+`<button class="iconbtn history-delete" data-delete-run="${esc(id)}" aria-label="删除 ${esc(r?.name||'执行记录')}" title="删除记录" ${!r?.finished?'disabled':''}>${icon('trash')}</button>`})
};
async function previewRemoveRuns(scope){
 if(mutating)return;mutating=true;const pending=toast('正在删除记录…','loader');
 try{const result=await api('delete-history',scope);applySnapshot(await api('state'));pending.remove();toast(t('history.deleted',{count:result.count}))}
 catch(error){pending.remove();toast('删除失败：'+error.message,'alert')}finally{mutating=false}
}
document.addEventListener('click',async e=>{
 const single=e.target.closest('[data-delete-run]'),bulk=e.target.closest('[data-clear-history]');if((!single&&!bulk)||(single||bulk).disabled)return;
 const filtered=bulk?.dataset.clearHistory==='filtered';
 const scope=single?{ids:[single.dataset.deleteRun]}:filtered?{filter:{date:filters.historyDate,query:filters.historyQuery,status:filters.historyStatus,timezone:state.settings.timezone}}:{all:true};
 try{const result=await api('delete-history',{...scope,preview:true});if(!result.count){toast('没有可删除的已结束记录','info');return}
 const title=single?'删除执行记录':filtered?'删除筛选结果':'删除全部记录';
 const summary=single?'仅删除这一条执行记录。':filtered?`日期：${filters.historyDate||'不限'}；关键词：${filters.historyQuery||'不限'}；执行结果：${document.querySelector('[data-filter="historyStatus"]')?.selectedOptions[0]?.textContent||'全部结果'}`:'覆盖所有日期和执行结果，忽略当前筛选条件。';
 modal(title,`<p>将删除 ${result.count} 条已结束的执行记录。</p><p class="history-delete-scope">${esc(summary)}</p><p class="small muted" data-i18n="message.928ee86f61b5">执行中的记录保留；账号、分组和定时任务不受影响。删除后无法恢复。</p>`,title,()=>previewRemoveRuns(scope),true)
 }catch(error){toast('无法读取删除范围：'+error.message,'alert')}
});
let activePreviewSelect=null,selectSeq=0;
function closePreviewSelect(focus=false){if(!activePreviewSelect)return;const a=activePreviewSelect;activePreviewSelect=null;a.panel.hidden=true;a.button.setAttribute('aria-expanded','false');if(focus&&a.button.isConnected)a.button.focus()}
function enhancePreviewSelects(){
 if(typeof document==='undefined'||!document?.body)return;
 selectObserver.disconnect();
 document.querySelectorAll('[data-zone-search]').forEach(el=>el.remove());
 document.querySelectorAll('select').forEach(select=>{
  if(select.closest('.spark-select'))return;
  const wrap=document.createElement('div');wrap.className='spark-select';if(/zone/i.test(select.id))wrap.classList.add('spark-zone-select');select.before(wrap);wrap.append(select);select.classList.add('spark-native');select.tabIndex=-1;select.setAttribute('aria-hidden','true');
  const button=document.createElement('button');button.type='button';button.className='field spark-select-trigger';button.setAttribute('aria-haspopup','listbox');button.setAttribute('aria-expanded','false');button.disabled=select.disabled;
  const label=select.getAttribute('aria-label')||(select.id?document.querySelector(`label[for="${select.id}"]`)?.textContent:null)||select.closest('label')?.textContent.trim()||'选择选项';button.setAttribute('aria-label',label);
  const panel=document.createElement('div');panel.className='spark-select-panel';panel.hidden=true;panel.id='spark-options-'+(++selectSeq);button.setAttribute('aria-controls',panel.id);
  const searchable=/zone/i.test(select.id);let search=null;
  if(searchable){search=document.createElement('input');search.type='search';search.className='field spark-select-search';search.placeholder=previewControlText('searchZone');search.setAttribute('aria-label',previewControlText('searchZone'));panel.append(search)}
  const list=document.createElement('div');list.className='spark-select-options';list.setAttribute('role','listbox');list.setAttribute('aria-label',label);panel.append(list);
  function sync(){button.innerHTML=`<span>${esc(select.selectedOptions[0]?.textContent||'请选择')}</span>${icon('chevron')}`;button.disabled=select.disabled;button.title=select.selectedOptions[0]?.textContent||''}
  function choose(option){select.value=option.value;closePreviewSelect();select.dispatchEvent(new Event('change',{bubbles:true}));sync();if(button.isConnected)button.focus()}
  function options(){const query=(search?.value||'').toLowerCase();list.innerHTML='';[...select.options].filter(o=>o.textContent.toLowerCase().includes(query)).forEach(o=>{const b=document.createElement('button');b.type='button';b.className='spark-select-option';b.setAttribute('role','option');b.setAttribute('aria-selected',String(o.selected));b.innerHTML=`<span>${esc(o.textContent)}</span><span class="spark-option-check" aria-hidden="true">${o.selected?icon('check'):''}</span>`;b.disabled=o.disabled;b.addEventListener('click',()=>choose(o));list.append(b)});if(!list.children.length)list.innerHTML=`<div class="spark-select-empty">${esc(previewControlText('emptyZone'))}</div>`}
  button.addEventListener('click',()=>{if(activePreviewSelect?.button===button){closePreviewSelect();return}closePreviewSelect();options();panel.hidden=false;button.setAttribute('aria-expanded','true');activePreviewSelect={button,panel};const r=button.getBoundingClientRect(),vw=window.innerWidth;const width=Math.min(Math.max(r.width,searchable?380:210),vw-24);panel.style.width=width+'px';panel.style.left=Math.max(12,Math.min(r.left,vw-width-12))+'px';const below=window.innerHeight-r.bottom-12;panel.style.maxHeight=Math.max(120,Math.min(320,Math.max(below,r.top-12)))+'px';panel.style.top=(below<180?Math.max(12,r.top-Math.min(320,r.top-12)):r.bottom+6)+'px';if(search){search.value='';options();search.focus()}else (list.querySelector('[aria-selected="true"]')||list.querySelector('button'))?.focus();list.querySelector('[aria-selected="true"]')?.scrollIntoView?.({block:'nearest'})});
  button.addEventListener('keydown',e=>{if(['ArrowDown','ArrowUp'].includes(e.key)){e.preventDefault();button.click()}});
  panel.addEventListener('keydown',e=>{if(e.key==='Escape'){e.stopPropagation();closePreviewSelect(true);return}if(!['ArrowDown','ArrowUp','Home','End'].includes(e.key))return;e.preventDefault();const bs=[...list.querySelectorAll('button:not(:disabled)')],at=bs.indexOf(document.activeElement);const next=e.key==='Home'?0:e.key==='End'?bs.length-1:e.key==='ArrowDown'?(at+1)%bs.length:(at-1+bs.length)%bs.length;bs[next]?.focus()});
  search?.addEventListener('input',options);select.addEventListener('change',sync);sync();wrap.append(button,panel);
 });
 if(activePreviewSelect&&!activePreviewSelect.button.isConnected)closePreviewSelect();
 selectObserver.observe(document.body,{childList:true,subtree:true});
}
const selectObserver=new MutationObserver(enhancePreviewSelects);
window.addEventListener('pagehide',()=>selectObserver.disconnect());
document.addEventListener('click',e=>{if(activePreviewSelect&&!e.target.closest('.spark-select'))closePreviewSelect()});
document.addEventListener('keydown',e=>{if(e.key==='Escape'&&activePreviewSelect){e.preventDefault();e.stopPropagation();closePreviewSelect(true)}},true);
document.addEventListener('scroll',e=>{if(activePreviewSelect&&!activePreviewSelect.panel.contains(e.target))closePreviewSelect()},true);
window.addEventListener('resize',()=>closePreviewSelect());


function previewControlText(key){const texts={
 'zh-CN':{searchZone:'搜索时区名称或 UTC 偏移…',emptyZone:'没有匹配的时区'},
 en:{searchZone:'Search timezone or UTC offset…',emptyZone:'No matching timezones'},
 'zh-TW':{searchZone:'搜尋時區名稱或 UTC 偏移…',emptyZone:'沒有符合的時區'},
 ru:{searchZone:'Поиск часового пояса или UTC…',emptyZone:'Часовые пояса не найдены'},
 vi:{searchZone:'Tìm múi giờ hoặc độ lệch UTC…',emptyZone:'Không tìm thấy múi giờ'},
 ko:{searchZone:'시간대 이름 또는 UTC 오프셋 검색…',emptyZone:'일치하는 시간대가 없습니다'}};
 return (texts[hostPreferences.language]||texts.en)[key]
}
// Reuse the app's shared notification entry point; no parent-window API is assumed.
toast=function(message,type='check'){
 const stack=document.getElementById('toastStack');stack.setAttribute('popover','manual');stack.removeAttribute('role');stack.setAttribute('aria-live','off');
 const el=document.createElement('div'),error=['alert','x','error'].includes(type),pending=['loader','loading'].includes(type);el.className='toast feedback-toast '+(error?'is-error':pending?'is-pending':type==='info'?'is-info':'is-success');el.setAttribute('role',error?'alert':'status');el.setAttribute('aria-live',error?'assertive':'polite');el.setAttribute('aria-atomic','true');
 el.innerHTML=`<span class="feedback-icon ${pending?'spin':''}">${icon(error?'alert':pending?'loader':type==='info'?'info':'check')}</span><span class="feedback-message">${esc(message)}</span><button type="button" class="feedback-dismiss" aria-label="关闭提示">${icon('x')}</button>`;
 stack.append(el);while(stack.children.length>3)stack.firstElementChild.remove();
 function remove(){el.remove();if(!stack.children.length&&stack.hidePopover)try{stack.hidePopover()}catch{}}
 el.querySelector('button').onclick=remove;if(stack.showPopover)try{stack.showPopover()}catch{}
 if(!error&&!pending){let timer=setTimeout(remove,4500);el.onmouseenter=()=>clearTimeout(timer);el.onmouseleave=()=>{timer=setTimeout(remove,2500)}}
 return {remove,element:el}
};
let previewSettingsFeedback='';
const settingsFeedbackBase=renderSettings;
renderSettings=function(){return settingsFeedbackBase().replace('>保存设置</button>',`>保存设置</button><p class="settings-save-feedback" role="status">${esc(previewSettingsFeedback)}</p>`)};
function previewConfigMessage(c){
 if(c.tasks.length<state.tasks.length)return '任务已删除';if(c.groups.length<state.groups.length)return '分组已删除';
 if(c.tasks.length>state.tasks.length)return '任务已创建';if(c.groups.length>state.groups.length)return '分组已创建';
 if(JSON.stringify(c.settings)!==JSON.stringify(state.settings))return document.getElementById('settingZone')?'设置已保存':c.settings.paused?'调度已暂停':'调度已恢复';
 if(JSON.stringify(c.groups)!==JSON.stringify(state.groups))return '分组已保存';if(JSON.stringify(c.tasks)!==JSON.stringify(state.tasks))return '任务已保存';
 if(JSON.stringify(c.overrides)!==JSON.stringify(state.overrides))return '账号预热设置已保存';return '设置已保存'
}
commitConfig=async function(config){
 if(mutating)return;mutating=true;const message=previewConfigMessage(config),settings=!!document.getElementById('settingZone'),button=document.querySelector('[data-action="save-settings"]');if(button){button.disabled=true;button.textContent='保存中…'}
 const pending=toast('正在保存…','loader');
 try{const snapshot=await api('config',config);settingsDraft=null;if(settings)previewSettingsFeedback='已保存 · '+previewTimestamp(Date.now());applySnapshot(snapshot);pending.remove();toast(message);return true}
 catch(error){pending.remove();if(settings){previewSettingsFeedback='保存失败，请重试';const feedback=document.querySelector('.settings-save-feedback');if(feedback)feedback.textContent=previewSettingsFeedback}toast('保存失败：'+error.message,'alert');return false}
 finally{mutating=false;if(button?.isConnected){button.disabled=false;button.textContent='保存设置'}}
};
const warmupFeedbackBase=confirmQuickWarmup;
confirmQuickWarmup=function(button){const q=quickWarmup;if(!q||button.disabled)return;const count=q.ids.map(account).filter(a=>a&&a.provider===q.provider&&quickEligible(a)).length;if(!count){toast('没有可执行账号，请检查账号状态','info');return}warmupFeedbackBase(button)};
render();enhancePreviewSelects();

const productSnapshotBase=applySnapshot;
applySnapshot=function(snapshot,redraw=true){const pending=new Set(state.runs.filter(r=>!r.finished).map(r=>r.id));productSnapshotBase(snapshot,redraw);for(const r of state.runs){if(!pending.has(r.id)||!r.finished)continue;const ok=r.items.filter(i=>i.status==='success').length,failed=r.items.filter(i=>i.status==='failed').length,unknown=r.items.filter(i=>i.status==='unknown').length;toast(r.name+'：'+ok+' 成功'+(failed?'，'+failed+' 失败':'')+(unknown?'，'+unknown+' 未确认':''),failed||unknown?'alert':'check')}};

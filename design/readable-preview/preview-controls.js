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
 return clearHistoryBase().replace('<button class="btn" data-action="export-history">',`<button class="btn" data-clear-history="filtered" ${!count?'disabled':''}>${icon('trash')}删除筛选结果 <span class="badge">${count}</span></button><button class="btn danger" data-clear-history="all" ${!all?'disabled':''}>删除全部</button><button class="btn" data-action="export-history">`).replace('<section class="panel">','<p class="history-clear-note">删除筛选结果：按当前日期、关键词和执行结果筛选。删除全部：忽略筛选条件。执行中的记录会保留。</p><section class="panel">').replace(/(<button class="iconbtn" data-action="run-detail" data-id="([^"]+)"[^>]*>[\s\S]*?<\/button>)/g,(_,button,id)=>{const r=state.runs.find(r=>r.id===id);return button+`<button class="iconbtn history-delete" data-delete-run="${esc(id)}" aria-label="删除 ${esc(r?.name||'执行记录')}" title="删除记录" ${!r?.finished?'disabled':''}>${icon('trash')}</button>`})
};
async function previewRemoveRuns(scope){const result=await api('delete-history',scope);applySnapshot(await api('state'));toast(t('history.deleted',{count:result.count}))}
document.addEventListener('click',e=>{
 const single=e.target.closest('[data-delete-run]'),bulk=e.target.closest('[data-clear-history]');if((!single&&!bulk)||(single||bulk).disabled)return;
 if(single){const r=state.runs.find(r=>r.id===single.dataset.deleteRun);if(!r?.finished)return;modal('删除执行记录',`<p>删除「${esc(r.name)}」的这条执行记录？</p><p class="small muted">${previewTimestamp(r.started)} · 仅删除这一条记录。</p>`,'删除记录',()=>previewRemoveRuns([r.id]),true);return}
 const filtered=bulk.dataset.clearHistory==='filtered',ids=state.runs.filter(r=>r.finished&&(!filtered||previewHistoryMatches(r))).map(r=>r.id);
 const scope=filtered?`日期：${filters.historyDate||'不限'}；关键词：${filters.historyQuery||'不限'}；执行结果：${document.querySelector('[data-filter="historyStatus"]')?.selectedOptions[0]?.textContent||'全部结果'}`:'覆盖所有日期和执行结果，忽略当前筛选条件。';
 modal(filtered?'删除筛选结果':'删除全部记录',`<p>将删除 ${ids.length} 条已结束的执行记录。</p><p class="history-delete-scope">${esc(scope)}</p><p class="small muted">执行中的记录保留；账号、分组和定时任务不受影响。删除后无法恢复。</p>`,filtered?'删除筛选结果':'删除全部',()=>previewRemoveRuns(ids),true)
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

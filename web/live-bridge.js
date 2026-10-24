// Production bridge: the server owns configuration, time, queue and results.
let managementKey='',observedKey='',connectionAttempt=0,serverRevision=0,serverState=null,mutating=false,connected=false,polling=false;
const mount=CPAPluginSession.resourcePrefix(window);
const apiBase=mount+'/v0/management/plugins/cpa-spark/';
// Connect silently. Connection errors use the shared feedback component.
let connectionStatus='',lastConnectionError='',connectionPending=false,nextConnectionRetryAt=0;
document.body.dataset.disconnected='true';
function reportConnectionError(message){connectionStatus=message;if(message&&message!==lastConnectionError){lastConnectionError=message;toast(message,'alert')}}
function disconnectLive(message='',notify=true){connected=false;connectionPending=false;managementKey='';connectionAttempt++;serverState=null;serverRevision=0;state.accounts=[];state.tasks=[];state.groups=[];state.runs=[];activeRuns.clear();selected.clear();settingsDraft=null;document.querySelectorAll('dialog[open]').forEach(d=>d.close());document.body.dataset.disconnected='true';render();connectionStatus=message;if(notify)reportConnectionError(message)}
function api(endpoint,data){return authenticatedAPI(apiBase+endpoint,data)}
function cpaAPI(path){return authenticatedAPI(mount+'/v0/management/'+path)}
async function authenticatedAPI(url,data){
 const key=CPAPluginSession.savedConnection(window)?.managementKey;
 if(!key||key!==managementKey)throw Error('CPA 登录状态已变化，请重新连接');
 const response=await fetch(url,{method:data===undefined?'GET':'POST',cache:'no-store',credentials:'same-origin',headers:{'Authorization':'Bearer '+key,'Content-Type':'application/json'},body:data===undefined?undefined:JSON.stringify(data)});
 const body=await response.json().catch(()=>({error:'Invalid CPA response'}));
 if(CPAPluginSession.savedConnection(window)?.managementKey!==key||managementKey!==key)throw Error('CPA 登录状态已变化，请重新连接');
 if(response.status===401||response.status===403){disconnectLive('CPA 登录已失效，请返回 CPA 重新登录');throw Error('CPA 登录已失效')}
 if(!response.ok)throw Error(body.error||'HTTP '+response.status);return body
}
function currentConfig(){return {revision:serverRevision,tasks:clone(state.tasks),groups:clone(state.groups),settings:clone(state.settings),overrides:clone(state.overrides)}}
function applySnapshot(snapshot,redraw=true){const badge=document.querySelector('.plugin-version');if(badge){badge.setAttribute('translate','no');badge.textContent='v'+snapshot.version}serverState=snapshot;serverRevision=snapshot.config.revision;const c=snapshot.config;state={...state,...clone(c),runs:snapshot.runs,todayStats:snapshot.todayStats};activeRuns.clear();for(const run of state.runs)if(!run.finished)activeRuns.set(run.taskId,run.id);if(redraw){const scroll=$('.wt-scroll')?.scrollLeft||0;render();if($('.wt-scroll'))$('.wt-scroll').scrollLeft=scroll;if(detailType==='run'&&$('#detailDialog').open){const run=state.runs.find(r=>r.id===detailId);if(run&&!run.summary)renderRunDetail(run)}};connectionStatus=snapshot.lastError?'STOPPED: '+snapshot.lastError:'Connected · server '+new Date(snapshot.now).toLocaleTimeString()}
async function refreshLive(force=false){if(!connected||polling||mutating)return;polling=true;try{const snap=await api('state');const editing=!!document.querySelector('dialog[open]')||$('#main').contains(document.activeElement);if(snap.config.revision!==serverRevision&&editing&&!force){connectionStatus='Server configuration changed · save will check for conflict';return}applySnapshot(snap,force||!editing||detailType==='run')}catch(error){connectionStatus=error.message}finally{polling=false}}
async function commitConfig(config){if(mutating)return;mutating=true;try{const snapshot=await api('config',config);settingsDraft=null;applySnapshot(snapshot);toast('Saved to CPA');return true}catch(error){toast(error.message,'alert');return false}finally{mutating=false}}
async function connectLive(){
 const session=CPAPluginSession.savedConnection(window);observedKey=session?.managementKey||'';
 if(!session){disconnectLive('请先在 CPA 登录并勾选“记住密码”，再返回插件');return}
 disconnectLive('',false);connectionPending=true;nextConnectionRetryAt=Date.now()+10000;managementKey=session.managementKey;const attempt=connectionAttempt;
 try{const [snapshot,as]=await Promise.all([api('state'),api('accounts')]);if(attempt!==connectionAttempt)return;state.accounts=as.accounts.map(a=>({...a,plan:a.plan||'未识别',tone:a.provider==='codex'?'orange':'green',status:a.disabled?'disabled':a.unavailable||a.status==='error'?'expired':'active'}));connected=true;document.body.dataset.disconnected='false';lastConnectionError='';applySnapshot(snapshot)}catch(error){if(attempt===connectionAttempt)disconnectLive(error.message)}finally{if(attempt===connectionAttempt)connectionPending=false}
}
function synchronizeLogin(){const key=CPAPluginSession.savedConnection(window)?.managementKey||'';if(key!==observedKey||(key&&!connected&&!connectionPending&&Date.now()>=nextConnectionRetryAt))void connectLive()}
window.addEventListener('storage',synchronizeLogin);
window.addEventListener('focus',synchronizeLogin);
document.addEventListener('visibilitychange',()=>{if(!document.hidden)synchronizeLogin()});
startRun=async function(t){if(mutating||!connected)return;mutating=true;const key=crypto.randomUUID();try{const result=await api('run',{taskId:t.id,key});applySnapshot(await api('state'));showRun(result.runId);toast('Request queued on CPA')}catch(error){toast(error.message,'alert')}finally{mutating=false}};
persist=function(){throw Error('Client persistence is disabled in live mode')};
pumpSimulations=function(){};
const mutations=new Set(['save-task','save-group','global-pause','toggle-task','bulk','duplicate-task','delete-task','delete-group','pause-account','save-settings','cancel-run','reset-demo','export-config','about']);
document.addEventListener('click',async event=>{
 const el=event.target.closest('[data-action]');if(!el||!mutations.has(el.dataset.action))return;event.stopImmediatePropagation();event.preventDefault();if(!connected)return;if(mutating){toast('Please wait for the current save','info');return}
 const action=el.dataset.action,id=el.dataset.id,c=currentConfig();openMenu=null;document.querySelectorAll('body > .row-menu').forEach(m=>m.remove());
 if(action==='save-task'){const error=validateDraft();if(error)return taskError(error);const t=clone(draft);delete t._modelChosen;t.times=[...new Set(t.times)].sort();t.days=[...new Set(t.days)].sort();const index=c.tasks.findIndex(v=>v.id===t.id);t.revision=(index>=0?c.tasks[index].revision:0)+1;if(index>=0)c.tasks[index]=t;else c.tasks.unshift(t);if(await commitConfig(c))closeDialog('taskDialog');return}
 if(action==='save-group'){readGroupForm();const g=clone(groupDraft);g.name=g.name.trim();if(!g.name||!g.members.length){$('#groupError').textContent='Name and at least one account required';return}const i=c.groups.findIndex(v=>v.id===g.id);if(i>=0)c.groups[i]=g;else c.groups.push(g);if(await commitConfig(c))closeDialog('groupDialog');return}
 if(action==='global-pause')c.settings.paused=!c.settings.paused;
 if(action==='toggle-task'){const t=c.tasks.find(t=>t.id===id);t.enabled=!t.enabled;t.revision++}
 if(action==='bulk'){c.tasks.forEach(t=>{if(selected.has(t.id)){t.enabled=id==='enable';t.revision++}});selected.clear()}
 if(action==='duplicate-task'){const t=clone(task(id));t.id=crypto.randomUUID();t.name+=' · copy';t.enabled=false;t.revision=1;c.tasks.unshift(t)}
 if(action==='pause-account')c.overrides[id]=!c.overrides[id];
 if(action==='delete-task'){modal('Delete task?', '<p>Future scheduled requests will stop. Existing records and submitted requests remain.</p>','Delete',async()=>{c.tasks=c.tasks.filter(t=>t.id!==id);await commitConfig(c)},true);return}
 if(action==='delete-group'){modal('Delete group?', '<p>References will be removed. Tasks with no remaining targets will be paused; execution history remains.</p>','Delete',async()=>{c.groups=c.groups.filter(g=>g.id!==id);c.tasks=c.tasks.map(t=>({...t,groupIds:t.groupIds.filter(g=>g!==id),revision:t.revision+1})).map(t=>{const set=new Set(t.accountIds);for(const g of c.groups)if(t.groupIds.includes(g.id))g.members.forEach(a=>set.add(a));t.excludes.forEach(a=>set.delete(a));if(!set.size)t.enabled=false;return t});if(await commitConfig(c))closeDialog('groupDialog')},true);return}
 if(action==='save-settings'){c.settings={...c.settings,timezone:$('#settingZone').value,concurrency:Number($('#settingConcurrency').value),late:Number($('#settingLate').value),retention:Number($('#settingRetention').value)}}
 if(action==='cancel-run'){try{await api('cancel',{runId:id});await refreshLive(true)}catch(error){toast(error.message,'alert')};return}
 if(action==='reset-demo'){toast('Reset is disabled in the production plugin','info');return}
 if(action==='export-config'){download('cpa-spark-config.json',c);return}
 if(action==='about'){modal('CPA Spark','<p>CPA Spark（星火）为 CLIProxyAPI 提供单账号与自定义分组的独立预热调度、计划预览和执行追踪。</p><p>Model requests consume account quota. Actual quota reset is unconfirmed unless observed upstream.</p>');return}
 await commitConfig(c);
},true);
// Bind server validation to the current task and current DOM, including catalog refresh.
let taskPreviewSequence=0,taskPreview={key:'',status:'idle',times:[],error:''};
function taskPreviewPayload(){const t=clone(draft);for(const key of Object.keys(t))if(key.startsWith('_'))delete t[key];return t}
function taskPreviewKey(){return draft?JSON.stringify(taskPreviewPayload()):''}
function invalidateTaskPreview(){taskPreviewSequence++;taskPreview={key:'',status:'idle',times:[],error:''}}
function taskPreviewReady(){return !!draft&&taskModelReady(draft)&&taskPreview.key===taskPreviewKey()&&taskPreview.status==='ready'}
function syncTaskPreviewButton(){const button=document.querySelector('#taskDialog [data-action="save-task"]');if(button)button.disabled=!taskPreviewReady()}
function paintTaskPreview(){
 const area=document.getElementById('taskDialog');if(!area.open||taskStep!==2||!draft)return;
 const target=area.querySelector('.preview-box');
 if(target&&taskPreview.key===taskPreviewKey()){
  if(taskPreview.status==='ready')target.outerHTML=renderTimePreview(taskPreview.times,draft.timezone);
  else if(taskPreview.status==='loading')target.innerHTML='<div class="task-preview-state" role="status">'+icon('loader')+'正在验证运行时间…</div>';
  else if(taskPreview.status==='error')target.innerHTML='<div class="task-preview-state error" role="alert">'+icon('info')+'<span>'+esc(taskPreview.error)+'</span><button class="btn compact" data-retry-task-preview>重试</button></div>';
 }
 syncTaskPreviewButton()
}
async function requestTaskPreview(force=false){
 if(!draft||taskStep!==2||!document.getElementById('taskDialog').open||!taskModelReady(draft)){syncTaskPreviewButton();return}
 const key=taskPreviewKey();if(!force&&taskPreview.key===key&&taskPreview.status!=='idle'){paintTaskPreview();return}
 const seq=++taskPreviewSequence;taskPreview={key,status:'loading',times:[],error:''};paintTaskPreview();
 try{const result=await api('preview',{task:taskPreviewPayload()});if(seq!==taskPreviewSequence||key!==taskPreviewKey())return;
  if(!Array.isArray(result.times)||!result.times.length||result.times.some(v=>!Number.isFinite(new Date(v).getTime())))throw Error('未获取到有效的运行时间');
  taskPreview={key,status:'ready',times:result.times.map(v=>new Date(v).getTime()),error:''}
 }catch(error){if(seq!==taskPreviewSequence||key!==taskPreviewKey())return;taskPreview={key,status:'error',times:[],error:'运行时间验证失败：'+error.message}}
 paintTaskPreview()
}
document.addEventListener('click',event=>{if(event.target.closest('[data-retry-task-preview]'))void requestTaskPreview(true)});
document.getElementById('taskDialog').addEventListener('close',invalidateTaskPreview);
// A freshly created step/dialog is rendered before showModal is called.
const showTaskDialogBase=showDialog;
showDialog=function(id){showTaskDialogBase(id);if(id==='taskDialog'&&taskStep===2)void requestTaskPreview()};
setInterval(()=>{synchronizeLogin();void refreshLive()},2500);

renderSettings=function(){const prefs=settingsDraft||state.settings;return heading('设置','调度在 CPA 服务端运行，语言和主题自动跟随宿主。')+`<div class="settings-grid"><section class="panel settings-card"><h2>调度偏好</h2><div class="settings-row"><label for="settingZone">默认时区</label><select id="settingZone" class="field">${zonesOptions(prefs.timezone)}</select></div><div class="settings-row"><label for="settingConcurrency">最大并发账号数</label><input id="settingConcurrency" type="range" min="1" max="8" value="${prefs.concurrency}"><span id="concurrencyValue">${prefs.concurrency}</span></div><div class="settings-row"><label for="settingLate">允许迟到时间</label><select id="settingLate" class="field">${[60,120,300].map(v=>`<option value="${v}" ${prefs.late===v?'selected':''}>${v/60} min</option>`).join('')}</select></div><div class="settings-row"><label for="settingRetention">执行记录保留</label><select id="settingRetention" class="field">${[7,30,90].map(v=>`<option value="${v}" ${prefs.retention===v?'selected':''}>${v} days</option>`).join('')}</select></div><button class="btn primary" data-action="save-settings">保存偏好</button></section><section class="panel settings-card"><h2>语言与主题</h2><div class="detail-row"><span>界面语言</span><span id="hostLanguageValue" translate="no">${CPA_LANGUAGE_NAMES[hostPreferences.language]}</span></div><div class="detail-row"><span>主题模式</span><span id="hostThemeValue">${hostThemeLabel()}</span></div><div class="detail-row"><span>偏好来源</span><span id="hostSourceValue">${hostPreferenceDescription()}</span></div><p class="small muted">在 CPA 中修改语言或主题即可</p></section><section class="panel settings-card"><h2>服务端数据</h2><p class="small muted">SQLite · server scheduling · exact auth_id</p><button class="btn" data-action="export-config">导出配置</button><p class="small muted">自动复用 CPA 已保存的登录信息，无需在插件中填写密钥。</p></section></div>`};
render();
void connectLive();

// History presentation is independent from task editing and modal ownership.
filters.historyMode='account';filters.historyTask='';
let historyPage=1,historyPageSize=50,historyKey='',historyPendingKey='',historyGeneration=0,historyPromise=null,historyError='';
let historyData={rows:[],total:0,deletable:0,allDeletable:0,page:1,pages:1},historySearchTimer=null;
let accountTimelineRuns=null,accountTimelineAccount='',accountTimelineGeneration=0,runDetailGeneration=0;
function historyFilter(){const query=filters.historyQuery||'';return {query,date:filters.historyDate||'',status:filters.historyStatus||'all',timezone:state.settings.timezone,taskId:filters.historyTask||'',accountMatches:query?state.accounts.filter(a=>[a.id,a.name,a.email].join(' ').toLowerCase().includes(query.toLowerCase())).map(a=>a.id):[]}}
function historyQuery(){return {mode:filters.historyMode,filter:historyFilter(),page:historyPage,pageSize:historyPageSize}}
async function requestHistory(force=false){
 if(!connected||view!=='history')return;
 const query=historyQuery(),key=JSON.stringify(query);
 if(historyPendingKey===key)return historyPromise;
 if(!force&&historyKey===key)return;
 const generation=++historyGeneration;historyPendingKey=key;historyError='';
 historyPromise=(async()=>{try{const result=await api('history',query);if(generation!==historyGeneration||!connected)return;
  if(!Array.isArray(result.rows)||!Number.isInteger(result.total)||!Number.isInteger(result.page))throw Error(t('history.invalidResponse'));
  historyData=result;historyPage=result.page;historyKey=JSON.stringify({...query,page:result.page});
 }catch(error){if(generation===historyGeneration){historyError=error.message;historyKey=key}}
 finally{if(generation===historyGeneration){historyPendingKey='';if(view==='history')render()}}})();return historyPromise;
}
function historyRows(){return !historyError&&historyKey===JSON.stringify(historyQuery())?historyData.rows.map(row=>({...row,account:account(row.authId)})):[]}
function historyEnded(row){return !['queued','running'].includes(row.status)}
function resetHistoryPage(){historyPage=1;historyKey='';historyError='';render()}
renderHistory=function(){
 const byAccount=filters.historyMode!=='task',rows=historyRows(),ready=historyKey===JSON.stringify(historyQuery())&&!historyError;
 const count=ready?historyData.deletable:0,all=ready?historyData.allDeletable:0;
 if(!ready&&!historyError)void requestHistory();
 const statuses=byAccount?['success','failed','skipped','unknown','running','queued']:['success','failed','partial','skipped','unknown','running'];
 const toolbar=`<div class="toolbar history-toolbar"><select class="field" id="historyMode" data-history-mode aria-label="${esc(t('history.mode'))}"><option value="account" ${byAccount?'selected':''}>${t('history.byAccount')}</option><option value="task" ${!byAccount?'selected':''}>${t('history.byTask')}</option></select><div class="search">${icon('search')}<input class="field" type="search" id="historySearch" data-filter="historyQuery" aria-label="${esc(t('history.search'))}" placeholder="${esc(t('history.search'))}" value="${esc(filters.historyQuery)}"></div><input class="field" type="date" data-filter="historyDate" aria-label="${esc(t('history.date'))}" value="${esc(filters.historyDate)}"><select class="field" data-filter="historyStatus" aria-label="${esc(t('history.status'))}"><option value="all">${t('history.allStatus')}</option>${statuses.map(s=>`<option value="${s}" ${s===filters.historyStatus?'selected':''}>${t('history.status.'+s)}</option>`).join('')}</select></div>`;
 const taskFilter=filters.historyTask?`<div class="history-task-filter"><span>${t('history.taskFilter')} <strong translate="no">${esc(task(filters.historyTask)?.name||filters.historyTask)}</strong></span><button class="btn compact" data-history-clear-task>${t('history.clearTask')}</button></div>`:'';
 const body=rows.map(row=>{
  const status=row.status,id=esc(row.id),name=byAccount?row.account?.name||row.authId:row.name;
  const title=byAccount?`<span translate="no">${esc(name)}</span><div class="task-meta" translate="no">${esc(row.account?.email||row.authId)}</div>`:`<button class="name-link" data-action="run-detail" data-id="${id}" translate="no">${esc(name)}</button><div class="task-meta mono" translate="no">${id}</div>`;
  const sources=byAccount?row.runs.map(r=>r.name).filter((v,i,a)=>a.indexOf(v)===i).join(' / '):t('history.accounts',{count:row.accountCount||0});
  return `<tr data-history-row="${id}"><td>${title}</td><td><span class="mono">${previewTimestamp(row.started)}</span><div class="task-meta mono" translate="no">${esc(row.model)}</div></td><td ${byAccount?'translate="no"':''}>${esc(sources)}</td><td>${statusBadge(status,byAccount?t('history.status.'+status):null)}</td><td><button class="iconbtn" data-action="run-detail" data-id="${esc(byAccount?row.runId:row.id)}" aria-label="${esc(t('history.details'))}">${icon('chevron')}</button><button class="iconbtn history-delete" ${byAccount?'data-delete-execution':'data-delete-run'}="${id}" ${!historyEnded(row)?'disabled':''} aria-label="${esc(t('history.delete'))}">${icon('trash')}</button></td></tr>`;
 }).join('');
 return heading(t('history.title'),'',`<button class="btn" data-clear-history="filtered" ${!count?'disabled':''}>${icon('trash')}${t('history.clearFiltered')} <span class="badge">${count}</span></button><button class="btn danger" data-clear-history="all" ${!all?'disabled':''}>${t('history.clearAll')}</button><button class="btn" data-history-export>${icon('download')}${t('history.export')}</button>`)+taskFilter+`<section class="panel history-panel">${toolbar}<div class="table-scroll"><table class="history-table"><thead><tr><th>${t(byAccount?'history.account':'history.task')}</th><th>${t('history.time')}</th><th>${t(byAccount?'history.sources':'history.results')}</th><th>${t('history.status')}</th><th>${t('history.actions')}</th></tr></thead><tbody>${body}</tbody></table></div>${historyError?`<div class="history-load-error" role="alert">${esc(t('history.loadFailed',{error:historyError}))}<button class="btn compact" data-history-retry>${t('history.retry')}</button></div>`:!ready?empty(t('history.loading'),''):!rows.length?empty(t('history.empty'),''):''}<div class="table-bottom history-pagination"><span>${t(byAccount?'history.accountCount':'history.taskCount',{count:ready?historyData.total:0})}</span><div class="history-pager"><label for="historyPageSize">${t('history.perPage')}</label><select class="field" id="historyPageSize" data-history-size>${[20,50,100].map(n=>`<option value="${n}" ${n===historyPageSize?'selected':''}>${n}</option>`).join('')}</select><button class="btn compact" data-history-page="${historyPage-1}" ${!ready||historyPage<=1?'disabled':''}>${t('history.previous')}</button><span>${t('history.pageOf',{page:ready?historyPage:1,pages:ready?historyData.pages:1})}</span><button class="btn compact" data-history-page="${historyPage+1}" ${!ready||historyPage>=historyData.pages?'disabled':''}>${t('history.next')}</button></div></div></section>`;
};
document.addEventListener('change',event=>{
 const el=event.target;
 if(el.hasAttribute('data-history-mode')){event.stopImmediatePropagation();filters.historyMode=el.value;filters.historyStatus='all';resetHistoryPage()}
 else if(el.hasAttribute('data-history-size')){event.stopImmediatePropagation();historyPageSize=Number(el.value);resetHistoryPage()}
 else if(el.dataset.filter?.startsWith('history')){event.stopImmediatePropagation();clearTimeout(historySearchTimer);filters[el.dataset.filter]=el.value;resetHistoryPage()}
},true);
document.addEventListener('input',event=>{
 const el=event.target;if(!el.dataset.filter?.startsWith('history'))return;
 event.stopImmediatePropagation();filters[el.dataset.filter]=el.value;historyPage=1;historyKey='';clearTimeout(historySearchTimer);historySearchTimer=setTimeout(()=>{historyError='';render()},200);
},true);
document.addEventListener('click',async event=>{
 const el=event.target.closest('[data-action="task-history"],[data-history-clear-task],[data-delete-execution],[data-delete-run],[data-clear-history],[data-history-export],[data-history-page],[data-history-retry]');if(!el||el.disabled)return;
 event.preventDefault();event.stopImmediatePropagation();
 if(el.dataset.action==='task-history'){filters.historyMode='task';filters.historyTask=el.dataset.id;filters.historyQuery='';filters.historyDate='';filters.historyStatus='all';historyPage=1;historyKey='';historyError='';navigate('history');return}
 if(el.hasAttribute('data-history-clear-task')){filters.historyTask='';resetHistoryPage();return}
 if(el.hasAttribute('data-history-page')){historyPage=Number(el.dataset.historyPage);historyKey='';historyError='';render();return}
 if(el.hasAttribute('data-history-retry')){historyKey='';historyError='';render();return}
 if(el.hasAttribute('data-history-export')){void exportHistory();return}
 const byAccount=filters.historyMode!=='task',single=el.hasAttribute('data-delete-execution')||el.hasAttribute('data-delete-run'),filtered=el.dataset.clearHistory==='filtered';
 let scope;
 if(byAccount)scope={mode:'account',...(single?{executionIds:[el.dataset.deleteExecution]}:filtered?{filter:historyFilter()}:{all:true})};
 else scope=single?{ids:[el.dataset.deleteRun]}:filtered?{mode:'task',filter:historyFilter()}:{all:true};
 if(byAccount&&scope.executionIds?.length===0)return;
 try{
  const result=await api('delete-history',{...scope,preview:true});if(!connected)return;if(!result.count){toast('没有可删除的已结束记录','info');return}
  const title=single?t('history.delete'):filtered?t('history.clearFiltered'):t('history.clearAll');
  const summary=filtered?t('history.allPages')+' '+`日期：${filters.historyDate||'不限'}；关键词：${filters.historyQuery||'不限'}；执行结果：${document.querySelector('[data-filter="historyStatus"]')?.selectedOptions[0]?.textContent||'全部结果'}`:single?t(byAccount?'history.deleteAccountOnly':'history.deleteTaskOnly'):'覆盖所有日期和执行结果，忽略当前筛选条件。';
  modal(title,`<p>${t('history.confirmDelete',{count:result.count})}</p><p class="history-delete-scope">${esc(summary)}</p><p class="small muted">${t('history.keepActive')}</p>`,title,async()=>{await previewRemoveRuns(scope);historyKey='';await requestHistory(true)},true);
 }catch(error){toast('无法读取删除范围：'+error.message,'alert')}
},true);
const historyRunDetailBase=renderRunDetail;
renderRunDetail=function(run){historyRunDetailBase(run);if($('#accountDialog').open){$('#detailDialog .drawer-footer .flex').insertAdjacentHTML('afterbegin',`<button class="btn" data-action="close" data-id="detailDialog">${t('history.backToAccount')}</button>`)}};

async function exportHistory(){
 const query={...historyQuery(),page:1,pageSize:100};const pending=toast(t('history.exporting'),'loader'),records=new Map();
 try{for(let page=1;;page++){if(!connected)return;const result=await api('history',{...query,page});if(!connected)return;for(const row of result.rows)records.set(row.id,row);if(page>=result.pages||result.page<page)break}download('cpa-spark-history.json',{mode:query.mode,filter:query.filter,records:[...records.values()]})}
 catch(error){toast(t('history.loadFailed',{error:error.message}),'alert')}finally{pending.remove()}
}
const pagedShowRunBase=showRun;
showRun=async function(id){
 const local=state.runs.find(r=>r.id===id);if(local&&!local.summary){pagedShowRunBase(id);return}
 const generation=++runDetailGeneration;
 try{const run=await api('history-detail',{id});if(generation!==runDetailGeneration||!connected)return;detailType='run';detailId=id;renderRunDetail(run);if(!$('#detailDialog').open)showDialog('detailDialog')}
 catch(error){if(connected)toast(t('history.loadFailed',{error:error.message}),'alert')}
};
async function refreshAccountTimeline(id){
 const generation=++accountTimelineGeneration;
 try{const result=await api('account-timeline',{id});if(generation!==accountTimelineGeneration||!connected||accountDetailId!==id||!$('#accountDialog').open)return;accountTimelineAccount=id;accountTimelineRuns=result.runs;const scroll=$('#accountDialog .drawer-content')?.scrollTop||0;renderAccountDetail();if($('#accountDialog .drawer-content'))$('#accountDialog .drawer-content').scrollTop=scroll}
 catch(error){if(generation===accountTimelineGeneration&&connected)toast(t('history.loadFailed',{error:error.message}),'alert')}
}
const pagedShowAccountBase=showAccount;
showAccount=function(id){accountTimelineRuns=null;accountTimelineAccount=id;pagedShowAccountBase(id);if(!state.runs.some(r=>!r.summary))void refreshAccountTimeline(id)};
const pagedSnapshotBase=applySnapshot;
applySnapshot=function(...args){pagedSnapshotBase(...args);if(view==='history'&&historyPage===1)void requestHistory(true)};
const pagedDisconnectBase=disconnectLive;
disconnectLive=function(...args){historyGeneration++;runDetailGeneration++;accountTimelineGeneration++;historyPage=1;historyKey='';historyPendingKey='';historyError='';historyData={rows:[],total:0,deletable:0,allDeletable:0,page:1,pages:1};accountTimelineRuns=null;pagedDisconnectBase(...args)};
$('#detailDialog').addEventListener('close',()=>runDetailGeneration++);
$('#accountDialog').addEventListener('close',()=>{accountTimelineGeneration++;accountTimelineRuns=null});
let historyRefreshBusy=false;
setInterval(async()=>{
 if(!connected||historyRefreshBusy)return;historyRefreshBusy=true;
 try{if($('#detailDialog').open&&detailType==='run'){const id=detailId,generation=runDetailGeneration;const run=await api('history-detail',{id});if(connected&&generation===runDetailGeneration&&$('#detailDialog').open&&detailId===id)renderRunDetail(run)}}catch(error){}finally{historyRefreshBusy=false}
},2500);
render();

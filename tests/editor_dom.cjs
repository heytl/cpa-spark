const {JSDOM}=require('jsdom');
const fs=require('node:fs'),assert=require('node:assert/strict');
const root=require('node:path').resolve(__dirname,'..');
const html=fs.readFileSync(root+'/web/index.html','utf8'),code=fs.readFileSync(root+'/web/app.js','utf8');
const d=new JSDOM(html,{url:'https://qa.example/v0/resource/plugins/cpa-spark/status',runScripts:'outside-only',pretendToBeVisual:true});
const w=d.window,errors=[];w.addEventListener('error',e=>errors.push(e.error||e.message));
w.TextEncoder=TextEncoder;w.TextDecoder=TextDecoder;w.matchMedia=()=>({matches:false,addEventListener(){},removeEventListener(){}});
w.HTMLDialogElement.prototype.showModal=function(){this.open=true};w.HTMLDialogElement.prototype.close=function(){this.open=false;this.dispatchEvent(new w.Event('close'))};
w.localStorage.setItem('cli-proxy-language','zh-CN');
w.localStorage.setItem('isLoggedIn','true');w.localStorage.setItem('cli-proxy-auth',JSON.stringify({state:{apiBase:'https://qa.example',managementKey:'qa-only',rememberPassword:true}}));
const config={revision:0,tasks:[],groups:[],overrides:{},settings:{timezone:'Asia/Shanghai',concurrency:4,retention:30,late:120,paused:false}};
const models={codex:[{id:'gpt-6-luna',thinking:{levels:['high','low','none']}},{id:'gpt-6-sol',thinking:{levels:['low','high']}}],antigravity:[{id:'gemini-3-flash',thinking:{min:128,max:8192,zero_allowed:true}}]};
const unavailable=process.env.CPA_QA_UNAVAILABLE==='1';
const requests=[];
const accounts=[{id:'qa-codex',name:'QA Codex',provider:'codex',status:'active'},{id:'qa-ag',name:'QA Gemini',provider:'antigravity',status:'active'}];
if(unavailable){accounts.forEach(a=>{a.unavailable=true;a.status='error'});config.overrides['qa-codex']=true;config.overrides['qa-ag']=true}
let compactSnapshot=false,holdHistory=false,failHistory=false;const historyResolves=[];
let qaRuns=[],failSave=false,holdCatalog=false,holdPreviews=false,failPreview=false;const catalogResolves=[],previewResolves=[];
const snapshot=()=>({version:'test',now:new Date().toISOString(),config:JSON.parse(JSON.stringify(config)),runs:JSON.parse(JSON.stringify(compactSnapshot?qaRuns.slice(0,1).map(r=>({...r,items:[],summary:true})):qaRuns))});
w.fetch=async(url,options)=>{if(url.endsWith('/history')){if(holdHistory)await new Promise(resolve=>historyResolves.push(resolve));if(failHistory)return {ok:false,status:500,json:async()=>({error:'QA history failed'})}}if(holdCatalog&&(url.includes('/auth-files/models?')||url.includes('/model-definitions/')))await new Promise(resolve=>catalogResolves.push(resolve));if(url.endsWith('/preview')){if(holdPreviews)await new Promise(resolve=>previewResolves.push(resolve));if(failPreview)return {ok:false,status:500,json:async()=>({error:'QA preview failed'})}}requests.push(url);assert.equal(options.headers.Authorization,'Bearer qa-only');let body;
if(url.endsWith('/history'))body=w.eval('previewHistoryPage')(qaRuns,accounts,JSON.parse(options.body));else if(url.endsWith('/history-detail'))body=qaRuns.find(r=>r.id===JSON.parse(options.body).id);else if(url.endsWith('/account-timeline'))body={runs:qaRuns};else if(url.endsWith('/accounts'))body={accounts};else if(url.includes('/auth-files/models?'))body={models:models[url.includes('qa-codex')?'codex':'antigravity'].map(({id})=>({id}))};else if(url.includes('/model-definitions/'))body={models:models[url.split('/').at(-1)]};else if(url.endsWith('/preview'))body={times:[new Date(Date.now()+3600000).toISOString()]};else if(url.endsWith('/delete-history')){const req=JSON.parse(options.body);const selected=qaRuns.filter(r=>r.finished&&(req.all||req.ids?.includes(r.id)||req.filter&&(!req.filter.query||r.name.includes(req.filter.query))));body={count:selected.length};if(!req.preview)qaRuns=qaRuns.filter(r=>!selected.includes(r))}else if(url.endsWith('/run-once')){const req=JSON.parse(options.body);assert.equal(req.task.effort,'none');qaRuns.unshift({id:'once-test',taskId:'once-task',name:req.task.name,started:Date.now(),finished:false,model:req.task.model,mode:'manual',items:[{id:'xi',authId:'qa-codex',status:'running',message:'Running'}]});body={runId:'once-test'}}else if(url.endsWith('/config')){if(failSave)return {ok:false,status:500,json:async()=>({error:'QA save failure'})};const c=JSON.parse(options.body);Object.assign(config,c,{revision:c.revision+1});body=snapshot()}else body=snapshot();return {ok:true,status:200,json:async()=>body}};
const wait=()=>new Promise(resolve=>setImmediate(resolve));
const click=sel=>{const el=w.document.querySelector(sel);assert(el,'missing '+sel);el.click()};
const change=(sel,value)=>{const el=w.document.querySelector(sel);assert(el,'missing '+sel);el.value=value;el.dispatchEvent(new w.Event('change',{bubbles:true}))};
function checkTimelineDates(){
 const realNow=w.Date.now,schedule=w.eval('timelineRows')()[0].tasks[0],dailyDays=schedule.days;
 try{
  // Replay the old green Saturday, the failing Sunday, and Shanghai midnight boundaries.
  for(const [at,weekdayWindows] of [
   ['2026-10-24T11:01:48Z',1],
   ['2026-10-31T16:00:00Z',0],
   ['2026-11-01T13:59:00Z',0],
   ['2026-11-01T15:59:59Z',0],
   ['2026-11-01T16:00:00Z',4],
  ]){
   w.Date.now=()=>Date.parse(at);
   schedule.days=[1,2,3,4,5];w.eval('render()');
   assert.equal(new Set([...w.document.querySelectorAll('.st-window')].map(el=>el.dataset.id)).size,weekdayWindows,'weekday windows at '+at);
   schedule.days=dailyDays;w.eval('render()');
   assert.equal(new Set([...w.document.querySelectorAll('.st-window')].map(el=>el.dataset.id)).size,5,'daily windows at '+at);
  }
 }finally{
  w.Date.now=realNow;schedule.days=dailyDays;w.eval('render()');
 }
}
(async()=>{try{
w.eval(fs.readFileSync(root+'/design/readable-preview/preview-history.js','utf8')+'\n'+code);await wait();await wait();assert.equal(w.document.body.dataset.disconnected,'false');assert.equal(w.document.querySelector('.plugin-tools'),null);assert.equal(w.document.querySelector('#connectionBanner'),null);
click('[data-action="new-task"]');let el=w.document.querySelector('#taskName');el.value='QA task';click('[data-action="target-mode"][data-id="accounts"]');click('[data-draft-select="accountIds"][value="qa-codex"]');click('[data-action="task-next"]');assert.equal(w.document.querySelector('#taskZone').value,'Asia/Shanghai');assert(w.document.querySelector('#taskZone').options.length>500);
// The nonempty timeline assertion needs a schedule that also runs on Sundays.
click('[data-action="days-preset"][data-id="daily"]');
change('[data-clock-hour="0"]','07');change('[data-clock-minute="0"]','35');assert.equal(w.document.querySelector('[data-time-index="0"]').value,'07:35');click('[data-editor-five-hour]');assert.deepEqual(Array.from(w.document.querySelectorAll('[data-time-index]'),e=>e.value),['07:35','12:35','17:35','22:35']);click('[data-action="add-time"]');assert.equal(w.document.querySelectorAll('[data-clock-hour]').length,5);
click('[data-action="task-next"]');await wait();await wait();assert.equal(w.document.querySelector('#taskModel').tagName,'SELECT');assert.equal(w.document.querySelector('#taskModel').value,'gpt-6-luna');assert.equal(w.document.querySelector('#taskEffort').value,'none');assert.equal(w.document.querySelector('[data-action="save-task"]').disabled,false);
holdCatalog=true;holdPreviews=true;click('[data-refresh-models]');assert(w.document.querySelector('[data-action="save-task"]').disabled);assert(w.document.querySelector('#taskEffort').disabled);holdCatalog=false;catalogResolves.splice(0).forEach(resolve=>resolve());await wait();await wait();assert.equal(w.document.querySelector('#taskModel').value,'gpt-6-luna');assert.equal(w.document.querySelector('#taskEffort').value,'none');assert.equal(previewResolves.length,1);change('#taskModel','gpt-6-sol');assert.equal(previewResolves.length,2);previewResolves.shift()();await wait();assert(w.document.querySelector('[data-action="save-task"]').disabled);holdPreviews=false;previewResolves.shift()();await wait();await wait();assert.equal(w.document.querySelector('[data-action="save-task"]').disabled,false);
failPreview=true;click('[data-refresh-models]');await wait();await wait();assert(w.document.querySelector('[data-action="save-task"]').disabled);assert(w.document.querySelector('[data-retry-task-preview]'));failPreview=false;click('[data-retry-task-preview]');await wait();await wait();assert.equal(w.document.querySelector('[data-action="save-task"]').disabled,false);
change('#taskModel','gpt-6-sol');assert.equal(w.document.querySelector('#taskModel').value,'gpt-6-sol');assert.equal(w.document.querySelector('#taskEffort').value,'low');
await wait();await wait();click('[data-action="save-task"]');await wait();await wait();assert.equal(config.tasks.length,1);assert.equal(config.tasks[0].model,'gpt-6-sol');assert.equal(config.tasks[0].effort,'low');
click('[data-action="edit-task"]');click('[data-action="task-next"]');click('[data-action="task-next"]');await wait();assert.equal(w.document.querySelector('#taskModel').value,'gpt-6-sol');assert.equal(w.document.querySelector('#taskEffort').value,'low');click('[data-action="close"][data-id="taskDialog"]');
assert.deepEqual(config.tasks[0].days,[0,1,2,3,4,5,6]);
click('[data-action="menu"]');assert(w.document.querySelector('body > .row-menu'));assert(w.document.querySelector('body > .row-menu [data-action="delete-task"]'));w.document.body.click();assert.equal(w.document.querySelector('body > .row-menu'),null);
assert(w.document.querySelector('.spark-timeline'));assert.equal(w.document.querySelectorAll('.st-day-label,.st-lane-meta,.st-now-pill').length,0);assert(w.document.querySelector('.st-current-time time'));assert.equal(w.document.querySelectorAll('.st-guide').length,1);assert(new Set([...w.document.querySelectorAll('.st-window')].map(el=>el.dataset.id)).size>0);click('[data-action="timeline-hours"][data-id="48"]');assert.equal(w.document.querySelectorAll('.st-tick small').length,0);const midnightDates=[...w.document.querySelectorAll('.st-tick-date:not([aria-hidden])')];assert.equal(midnightDates.length,3);assert(midnightDates.every(el=>/^\d{4}-\d{2}-\d{2}$/.test(el.textContent)));assert.equal(new Set(midnightDates.map(el=>el.textContent)).size,3);assert.equal(w.document.querySelectorAll('.st-tick-time').length,9);assert.equal(w.document.querySelectorAll('.wt-trigger-time').length,0);click('[data-action="timeline-detail"]');assert(w.document.querySelector('#modalDialog').open);click('[data-action="close"][data-id="modalDialog"]');click('[data-action="timeline-hours"][data-id="24"]');
checkTimelineDates();

// Midnight is *not* a clipping boundary in 48-hour mode. It becomes a
// boundary only when changing to a 24-hour page that ends/starts there.
const localMidnight=Date.parse('2026-11-03T16:00:00Z'); // Shanghai Nov 4, 00:00
const fiveHour={start:localMidnight-3*3600000,end:localMidnight+2*3600000};
const beforeDay={start:localMidnight-24*3600000,end:localMidnight,zone:'Asia/Shanghai'};
const afterDay={start:localMidnight,end:localMidnight+24*3600000,zone:'Asia/Shanghai'};
const twoDays={start:beforeDay.start,end:afterDay.end,zone:'Asia/Shanghai'};
const merged=w.eval('sparkTimelinePieces')(fiveHour,twoDays);
assert.equal(merged.length,1,'48h should merge a window crossing midnight into one bar');
assert.equal(merged[0].start,fiveHour.start);
assert.equal(merged[0].end,fiveHour.end);
assert(merged[0].showLabel && !merged[0].split,'a complete cross-day window keeps its label');
const pct=ms=>(ms-twoDays.start)/(twoDays.end-twoDays.start)*100;
const lane={key:'window-test',windows:[fiveHour],events:[],policy:{kind:'five-hour'},unanchored:false,paused:false,model:'gpt-6-luna'};
const row={key:'window-test',label:'QA',color:'orange'};
const html=w.eval('sparkTimelineLane')(row,lane,twoDays,pct,localMidnight);
assert.equal((html.match(/class="st-window /g)||[]).length,1,'48h should create one timeline button, not two');
assert(html.includes('21:00')&&html.includes('02:00')&&html.includes('5h 预计窗口'),'merged bar includes full time range and caption');
const earlier=w.eval('sparkTimelinePieces')(fiveHour,beforeDay);
const later=w.eval('sparkTimelinePieces')(fiveHour,afterDay);
assert.equal(earlier.length,1);
assert.equal(later.length,1);
assert.equal(earlier[0].end,localMidnight,'24h before midnight clips right at viewport boundary');
assert.equal(later[0].start,localMidnight,'24h after midnight clips left at viewport boundary');
assert(earlier[0].showLabel,'longer visible 3h part keeps the label in 24h view');
assert(!later[0].showLabel,'shorter visible 2h part has no duplicate label in 24h view');
const outside=w.eval('sparkTimelinePieces')(fiveHour,{start:localMidnight+6*3600000,end:localMidnight+30*3600000,zone:'Asia/Shanghai'});
assert.equal(outside.length,0,'no ghost bar when the quota window is outside the view');
const whole=w.eval('sparkTimelinePieces')({start:localMidnight+6*3600000,end:localMidnight+11*3600000},afterDay);
assert.equal(whole.length,1);
assert.equal(whole[0].showLabel,true,'an entirely visible five-hour window must always have labels');
const firstTask=w.eval('timelineRows()')[0].tasks[0],savedStrategy=firstTask.strategy,savedTimes=[...firstTask.times];
firstTask.strategy='rolling';firstTask.times=['00:30'];
const nextDue=Date.now()+24*60*60*1000;
w.CPASparkRollingPlans={[firstTask.id]:[{taskId:firstTask.id,authId:'qa-codex',nextDue,source:'upstream',status:'scheduled'}]};
w.eval('render()');
let display=w.document.querySelector('.tasks-table .schedule-rolling');
assert(display,'rolling task should use next-trigger schedule formatting');
assert(display.textContent.includes('下次触发：'),'next trigger label is required');
assert(display.textContent.includes(w.eval('dateKey')(nextDue,firstTask.timezone)+' '+w.eval('time')(nextDue,firstTask.timezone)),'render live server time in task time zone, not bootstrap');
assert(w.document.querySelector('.tasks-table td:nth-child(3) .target-label')?.getAttribute('title'),'account label must provide complete hover text');
assert(w.document.querySelector('.tasks-table .schedule-rolling')?.nextElementSibling.textContent.includes('连续窗口'),'daily label must be replaced by continuous strategy');
assert(!/窗口时间估算|上游重置推算|首次启动计划/.test(w.document.querySelector('.tasks-table .schedule-rolling')?.nextElementSibling.textContent||''),'rolling task subtitle should remain concise');

const main=w.document.querySelector('.tasks-table .schedule-rolling');
const taskID=firstTask.id;
const accountID='qa-codex';
const futureDue=Date.now()+4*60*60*1000;
const missedDue=Date.now()-3*60*60*1000;
const futureLabel=w.eval('dateKey')(futureDue,firstTask.timezone)+' '+w.eval('time')(futureDue,firstTask.timezone);
// A snapshot may contain a cursor that has already elapsed. Never display it
// as a future appointment even if a browser input keeps focus.
w.CPASparkRollingPlans={[taskID]:[{taskId:taskID,authId:accountID,nextDue:missedDue,source:'upstream',status:'scheduled'}]};
main.tabIndex=0;main.focus();
w.eval('syncTaskScheduleUI()');
assert.equal(main.textContent,'待调度','expired rolling cursor must not be labelled as the next trigger');
assert(!main.textContent.includes('2026-11-03'),'expired clock time must disappear');
assert.equal(w.document.activeElement,main,'live updates should not lose task table focus');
// When the CPA server enters a waiting state, show its current waitUntil.
w.CPASparkRollingPlans={[taskID]:[{taskId:taskID,authId:accountID,nextDue:futureDue,status:'waiting',source:'upstream'}]};
w.eval('syncTaskScheduleUI()');
assert(main.textContent.includes(futureLabel),'actual server waiting deadline must replace old cursor in the task list');
assert.equal(w.document.querySelector('.upcoming-list')?.innerHTML,w.eval('renderUpcomingItems')(w.eval('allUpcoming')()),'sidebar must receive server-driven schedule changes');
assert(w.eval('allUpcoming')().some(entry=>Math.abs(entry.at-futureDue)<1000),'next triggers must include the real rolling time');
const timelineEvents=w.eval('timelineRangeOccurrences')(firstTask,Date.now(),Date.now()+48*3600000,[accountID]);
assert.deepEqual(timelineEvents,[futureDue],'timeline must follow one server rolling event, not repeat daily bootstrap');
// A later CPA snapshot must update text in place without repainting the
// focused table, including both the main row and next-trigger summary.
const movedDue=futureDue+2*60*60*1000;
w.CPASparkRollingPlans={[taskID]:[{taskId:taskID,authId:accountID,nextDue:movedDue,status:'scheduled',source:'upstream'}]};
w.eval('syncTaskScheduleUI()');
assert(main.textContent.includes(w.eval('dateKey')(movedDue,firstTask.timezone)),'changed server due replaces previous waiting deadline');
assert.equal(w.document.activeElement,main,'focused row persists after server due time moves');
w.CPASparkRollingPlans={[taskID]:[{taskId:taskID,authId:accountID,nextDue:missedDue,status:'running',source:'upstream'}]};
w.eval('syncTaskScheduleUI()');
assert.equal(main.textContent,'正在执行','a running task cannot claim a stale future trigger');
w.CPASparkRollingPlans={[taskID]:[{taskId:taskID,authId:accountID,nextDue:0,status:'due',source:'upstream'}]};
w.eval('syncTaskScheduleUI()');
assert.equal(main.textContent,'待调度','due cursor shows scheduling state rather than a stale time');
w.CPASparkSchedulerError='Persistent storage is unavailable';
w.eval('syncTaskScheduleUI()');
assert.equal(main.textContent,'调度已停止','a halted server must not pretend the overdue task is about to run');
w.CPASparkSchedulerError='';
main.blur();

firstTask.strategy=savedStrategy;firstTask.times=savedTimes;
w.CPASparkRollingPlans={};w.eval('render()');

click('[data-action="new-task"]');w.document.querySelector('#taskName').value='Gemini QA';change('#taskProvider','antigravity');click('[data-action="target-mode"][data-id="accounts"]');click('[data-draft-select="accountIds"][value="qa-ag"]');click('[data-action="task-next"]');click('[data-action="task-next"]');await wait();await wait();assert.equal(w.document.querySelector('#taskModel').value,'gemini-3-flash');assert.equal(w.document.querySelector('#taskEffort').value,'budget:0');assert.equal(w.document.querySelector('[data-action="save-task"]').disabled,false);
click('[data-action="close"][data-id="taskDialog"]');
click('[data-action="nav"][data-id="settings"]');await wait();assert(w.document.querySelector('#settingZone').closest('.spark-select').classList.contains('spark-zone-select'));
assert.deepEqual([...w.document.querySelectorAll('#settingLate option')].map(x=>Number(x.value)),[60,120,300,600,900,1800]);assert(w.document.querySelector('#settingLate option[value="600"]')?.textContent.includes('默认'));
const trigger=w.document.querySelector('#settingZone').closest('.spark-select').querySelector('button');trigger.click();assert.equal(w.document.querySelector('.spark-select-search').placeholder,'搜索时区名称或 UTC 偏移…');w.document.body.click();
change('#settingZone','UTC');click('[data-action="save-settings"]');await wait();await wait();assert.equal(config.settings.timezone,'UTC');assert(w.document.querySelector('.settings-save-feedback').textContent.includes('已保存'));assert(w.document.querySelector('#toastStack').textContent.includes('设置已保存'));
failSave=true;click('[data-action="save-settings"]');await wait();await wait();assert(w.document.querySelector('.settings-save-feedback').textContent.includes('保存失败'));assert(w.document.querySelector('.feedback-toast.is-error[role="alert"]'));failSave=false;
click('[data-action="nav"][data-id="accounts"]');click('[data-action="new-group"]');assert.equal(w.document.querySelector('#groupName').value,'账号分组 1');assert.equal(w.document.querySelectorAll('[data-action="group-color"]').length,7);click('[data-action="group-color"][data-id="cyan"]');assert.equal(w.document.querySelectorAll('[data-action="group-color"][aria-pressed="true"]').length,1);click('[data-group-member][value="qa-codex"]');click('[data-action="save-group"]');await wait();await wait();assert.equal(config.groups.at(-1).color,'cyan');
if(!unavailable){click('[data-action="account-tab"][data-id="accounts"]');click('[data-quick-warmup="account"][data-id="qa-codex"]');await wait();await wait();click('[data-quick-confirm]');await wait();await wait();assert(requests.some(url=>url.endsWith('/run-once')));assert(w.document.querySelector('#detailDialog').open);click('[data-action="close"][data-id="detailDialog"]');assert.equal(config.tasks.length,1)}
qaRuns=[{id:'nav-run',taskId:config.tasks[0].id,name:'QA task',started:Date.now()-60000,finished:true,model:'m',mode:'manual',items:[{id:'nav-x',authId:'qa-codex',status:'success',started:Date.now()-60000,finished:Date.now()-30000,message:'OK',latency:20,tokens:2}]},
{id:'merged-run',taskId:'another-task',name:'Another task',started:Date.now()-60000,finished:true,model:'m',mode:'manual',items:[{id:'nav-x',authId:'qa-codex',status:'success',started:Date.now()-60000,finished:Date.now()-30000,message:'OK',latency:20,tokens:2}]}];w.eval('applySnapshot')(snapshot());
w.eval('showAccount("qa-codex")');assert(w.document.querySelector('#accountDialog').open);w.document.querySelector('#accountDialog .drawer-content').scrollTop=85;
click('#accountDialog [data-action="run-detail"]');assert(w.document.querySelector('#detailDialog').open);assert(w.document.querySelector('#accountDialog').open);click('#detailDialog .drawer-footer [data-action="close"]');assert(!w.document.querySelector('#detailDialog').open);assert(w.document.querySelector('#accountDialog').open);assert.equal(w.document.querySelector('#accountDialog .drawer-content').scrollTop,85);
click('#accountDialog [data-action="run-detail"]');w.document.querySelector('#detailDialog').close();assert(w.document.querySelector('#accountDialog').open);click('#accountDialog [data-action="close"]');
click('[data-action="nav"][data-id="history"]');await wait();await wait();assert.equal(w.document.querySelector('#historyMode').value,'account');assert.equal(w.document.querySelectorAll('[data-history-row]').length,1);assert(w.document.querySelector('[data-delete-execution="nav-x"]'));
assert(w.document.querySelector('[data-history-row] [data-action="run-detail"] svg[data-icon="eye"]'),'history action uses eye icon');
qaRuns.forEach(r=>r.items.forEach(i=>{i.status='failed';i.executionStatus='failed'}));w.eval('applySnapshot')(snapshot());await wait();await wait();assert.equal(w.document.querySelector('[data-history-row] td:nth-child(4)').textContent.trim(),'失败');assert(w.document.querySelector('[data-clear-history="filtered"]').textContent.includes('删除筛选结果'));assert(w.document.querySelector('[data-clear-history="all"]').textContent.includes('删除全部'));
change('#historyMode','task');await wait();await wait();assert.equal(w.document.querySelectorAll('[data-history-row]').length,2);
click('[data-action="nav"][data-id="tasks"]');const scheduleText=config.tasks[0].times.join(' / ');assert.equal(w.document.querySelector('.schedule-main').textContent,scheduleText);assert.equal(w.document.querySelector('.schedule-main').title,scheduleText);assert(w.document.querySelector('.upcoming-title'));assert.equal(w.document.querySelector('.upcoming-title').tagName,'DIV');assert(!w.document.querySelector('.upcoming-title').hasAttribute('data-action'));click('.task-name[data-action="task-history"]');await wait();await wait();assert.equal(w.document.querySelector('#historyMode').value,'task');assert.equal(w.document.querySelectorAll('[data-history-row]').length,1);assert(!w.document.querySelector('#taskDialog').open);click('[data-history-clear-task]');await wait();await wait();assert.equal(w.document.querySelectorAll('[data-history-row]').length,2);
qaRuns=[{id:'one',name:'Match',started:Date.now(),finished:true,model:'m',mode:'manual',items:[]},{id:'two',name:'Other',started:Date.now(),finished:true,model:'m',mode:'manual',items:[]},{id:'active',name:'Active',started:Date.now(),finished:false,model:'m',mode:'manual',items:[]}];w.eval('applySnapshot')(snapshot());
click('[data-action="nav"][data-id="history"]');await wait();await wait();assert.equal(w.document.querySelectorAll('[data-delete-run]').length,3);assert(w.document.querySelector('[data-delete-run="active"]').disabled);
click('[data-delete-run="one"]');await wait();assert(w.document.querySelector('#modalDialog').textContent.includes('1 条'));click('[data-action="close"][data-id="modalDialog"]');assert.equal(qaRuns.length,3);
w.document.querySelector('[data-filter="historyQuery"]').value='Match';w.document.querySelector('[data-filter="historyQuery"]').dispatchEvent(new w.Event('input',{bubbles:true}));await new Promise(r=>setTimeout(r,250));await wait();click('[data-clear-history="filtered"]');await wait();assert(w.document.querySelector('#modalDialog').textContent.includes('关键词：Match'));click('[data-action="modal-confirm"]');await wait();await wait();assert.deepEqual(qaRuns.map(r=>r.id),['two','active']);
click('[data-clear-history="all"]');await wait();assert(w.document.querySelector('#modalDialog').textContent.includes('忽略当前筛选'));click('[data-action="modal-confirm"]');await wait();await wait();assert.deepEqual(qaRuns.map(r=>r.id),['active']);
// Paged requests stay independent from the compact home snapshot.
compactSnapshot=true;qaRuns=Array.from({length:125},(_,i)=>({id:'page-run-'+String(i).padStart(3,'0'),taskId:'paged',name:'Page fixture',model:'m',started:Date.now()-i*1000,finished:true,items:[{id:'page-x-'+i,authId:'qa-codex',status:'success',started:Date.now()-i*1000,message:'OK'}]}));w.eval('applySnapshot')(snapshot());
change('[data-filter="historyQuery"]','');change('#historyMode','account');await wait();await wait();assert.equal(w.document.querySelector('#historyPageSize').value,'50');assert.equal(w.document.querySelectorAll('[data-history-row]').length,50);assert(w.document.querySelector('.history-pagination').textContent.includes('125'));
change('#historyPageSize','20');await wait();await wait();assert.equal(w.document.querySelectorAll('[data-history-row]').length,20);click('[data-history-page="2"]');await wait();await wait();assert.equal(w.document.querySelector('[data-history-row]').dataset.historyRow,'page-x-20');
const requestsBeforeRefresh=requests.filter(u=>u.endsWith('/history')).length;w.eval('applySnapshot')(snapshot());await wait();assert.equal(requests.filter(u=>u.endsWith('/history')).length,requestsBeforeRefresh);assert.equal(w.document.querySelector('[data-history-row]').dataset.historyRow,'page-x-20');
failHistory=true;click('[data-history-page="3"]');await wait();await wait();assert(w.document.querySelector('[data-history-retry]'));assert.equal(w.document.querySelectorAll('[data-history-row]').length,0);failHistory=false;click('[data-history-retry]');await wait();await wait();assert.equal(w.document.querySelector('[data-history-row]').dataset.historyRow,'page-x-40');
click('[data-history-row="page-x-40"] [data-action="run-detail"]');await wait();await wait();assert(w.document.querySelector('#detailDialog').open);assert(w.document.querySelector('#detailDialog').textContent.includes('page-run-040'));click('#detailDialog [data-action="close"]');
// Obsolete page responses must not replace a new display mode.
holdHistory=true;click('[data-history-page="4"]');change('#historyMode','task');assert.equal(historyResolves.length,2);historyResolves.shift()();await wait();assert.equal(w.document.querySelectorAll('[data-history-row]').length,0);holdHistory=false;historyResolves.shift()();await wait();await wait();assert.equal(w.document.querySelector('#historyMode').value,'task');assert.equal(w.document.querySelector('[data-history-row]').dataset.historyRow,'page-run-000');
change('#historyMode','account');await wait();await wait();assert(w.document.querySelector('[data-clear-history="filtered"]').textContent.includes('125'));click('[data-clear-history="filtered"]');await wait();assert(w.document.querySelector('#modalDialog').textContent.includes('所有匹配页'));assert(w.document.querySelector('#modalDialog').textContent.includes('125'));click('[data-action="modal-confirm"]');await wait();await wait();assert.equal(qaRuns.length,0);assert.equal(w.document.querySelectorAll('[data-history-row]').length,0);
// Regression: refreshing an open execution detail must retain its scroll position.
const scrollFixture={id:'scroll-run',name:'scroll test',mode:'manual',model:'gpt-6-luna',started:Date.now(),finished:false,items:[{id:'scroll-x',authId:'qa-codex',status:'running',message:'Checking quota',scheduled:Date.now(),waitUntil:Date.now()+30000,windowStatus:'waiting'}]};
const scrollDialog=w.document.querySelector('#detailDialog');
w.eval('renderRunDetail')(scrollFixture);scrollDialog.showModal();
scrollDialog.querySelector('.drawer-content').scrollTop=219;
w.eval('renderRunDetail')({...scrollFixture,finished:true,items:[{...scrollFixture.items[0],status:'success',message:'Completed',windowStatus:'unconfirmed'}]});
assert.equal(scrollDialog.querySelector('.drawer-content').scrollTop,219,'run detail auto-refresh must preserve user scroll');

const waitingRun={...scrollFixture,finished:false,items:[{...scrollFixture.items[0],status:'waiting',windowStatus:'waiting',windowResetAt:Date.now()+3600000,waitUntil:Date.now()+3605000,message:'Waiting for quota window to reset'}]};
w.eval('renderRunDetail')(waitingRun);
assert.equal(w.eval('runStatus')(waitingRun),'waiting','a waiting batch is not actively executing model requests');
assert(scrollDialog.querySelector('.status-title').textContent.includes('等待额度窗口重置'),'waiting modal displays a waiting headline');
assert.equal(scrollDialog.querySelector('.progressbar span').style.width,'0%','waiting account is not counted as completed');
assert(scrollDialog.querySelector('.run-item .subline').textContent.includes('等待至'),'pending window time is shown in the configured local timezone');
assert(scrollDialog.querySelector('.run-item .subline').textContent.includes('后检查窗口'),'waiting message should not show raw UTC protocol string');
assert(!scrollDialog.querySelector('.run-item .subline').textContent.includes('Waiting for quota window'),'raw UTC provider message must not be displayed');

const waitingWindow={...scrollFixture,finished:false,items:[{...scrollFixture.items[0],status:'waiting',waitUntil:Date.now()+2*60*60*1000,message:'Waiting for quota window'}]};
const signature=w.eval('runDetailFingerprint'),delay=w.eval('runDetailPollDelay');
assert.equal(signature(waitingWindow),signature({...waitingWindow,now:Date.now()+2500,serverNow:Date.now()+2500}),'changing server clock must not redraw waiting details');
assert.notEqual(signature(waitingWindow),signature({...waitingWindow,items:[{...waitingWindow.items[0],status:'running'}]}),'a genuine status change must redraw');
assert.equal(delay(waitingWindow,Date.now()),30000,'hours-long quota wait polls infrequently');
const almostDue={...waitingWindow,items:[{...waitingWindow.items[0],waitUntil:Date.now()+15000}]};
assert.equal(delay(almostDue,Date.now()),2500,'quota wait close to reset restores fast polling');
assert.equal(delay(scrollFixture,Date.now()),2500,'active requests continue fast polling');

scrollDialog.close();
const timelineCSS=fs.readFileSync(root+'/web/style.css','utf8');
assert(timelineCSS.includes('.st-table.st-hours-48{--st-plot-min:1536px}'),'48h timeline keeps five-hour bars readable');
assert(timelineCSS.includes('.st-table.st-hours-24{--st-plot-min:768px}'),'24h timeline keeps five-hour bars readable');
assert(timelineCSS.includes('.st-window--no-label .st-window-time'),'only shorter cross-day fragments hide labels');
assert(!timelineCSS.includes('.st-window{container-type:inline-size}'),'remove fragile container query hiding rules');
assert(!timelineCSS.includes('.st-window .st-window-time,.st-window .st-window-caption,.st-window .st-window-end{display:none!important}'), 'CSS specificity regression must not return');
assert(w.eval('sparkTimelineSummary').toString().includes('此时段无后续计划'), 'past planned windows are not misreported as no upstream window');
assert(!w.eval('sparkTimelineLane').toString().includes("right-left<16"),'percentage-based label hiding must be removed');

// Continuous mode shows an estimated five-hour sequence rather than five
// identical daily times. Disabled tasks remain saved but are not dispatched.
click('[data-action="nav"][data-id="tasks"]');
click('[data-action="new-task"]');
click('[data-action="target-mode"][data-id="accounts"]');
click('[data-draft-select="accountIds"][value="qa-codex"]');
click('[data-action="task-next"]');
assert.equal(w.document.querySelector('#taskStrategy').value,'fixed');
change('#taskStrategy','rolling');
assert.equal(w.document.querySelector('#taskStrategy').value,'rolling');
const projected=[...w.document.querySelectorAll('#taskDialog .preview-row .mono')].map(el=>el.textContent.trim());
assert.equal(projected.length,5,'rolling task should display five estimated times');
assert.equal(new Set(projected.map(t=>t.slice(0,5))).size,5,'rolling preview must not repeat the same clock time each day');
assert(w.document.querySelector('#taskDialog .spark-rolling-preview-hint')?.textContent.includes('推算'),'rolling preview must label unverified estimates');
click('[data-action="close"][data-id="taskDialog"]');
assert(requests.some(url=>url.includes('/auth-files/models?name=qa-codex')));assert(requests.some(url=>url.includes('/model-definitions/codex')));assert.deepEqual(errors,[]);console.log(unavailable?'PASS: paused/unavailable targets still load catalogs and reasoning without executing requests':'PASS: complete page bootstrap, centered-dialog markup, time editing, 596 zones, catalog defaults, model switch, save/edit preservation, menu portal and Gemini zero-budget');
}finally{w.dispatchEvent(new w.Event('pagehide'));await wait();d.window.close()}})().catch(e=>{console.error(e);process.exitCode=1});

// Preview-only presentation changes; server code and plugin build stay untouched.
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
openRunPreview=function(){toast('静态预览，不发送模型请求','info')};
startRun=async function(){toast('静态预览，不发送模型请求','info')};
render();
const centeredRunDetailBase=renderRunDetail;
renderRunDetail=function(run){document.getElementById('detailDialog').classList.add('execution-detail');centeredRunDetailBase(run)};
const originalAccountDetail=renderAccountDetail;
renderAccountDetail=function(){document.getElementById('detailDialog').classList.remove('execution-detail');originalAccountDetail()};

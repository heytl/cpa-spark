// Smart quota scheduling controls. Legacy tasks remain fixed-time by default.
(() => {
 const tr = (cn,en)=>typeof hostPreferences!=="undefined"&&String(hostPreferences.language).startsWith("zh")?cn:en;
 const original=renderTaskDialog;
 renderTaskDialog=function(){
  if(!draft)return;
  if(draft.strategy!=="rolling")draft.strategy="fixed";
  if(draft.strategy==="rolling"&&draft.times.length>1)draft.times=draft.times.slice(0,1);
  original();
  if(taskStep!==1)return;
  const dialog=document.querySelector("#taskDialog");
  const list=dialog?.querySelector(".time-list");
  const zone=dialog?.querySelector("#taskZone");
  if(!list)return;
  const field=document.createElement("div");
  field.className="field-group spark-strategy-field";
  field.innerHTML='<label class="label" for="taskStrategy">'+tr("调度策略","Scheduling strategy")+'</label><select class="field" id="taskStrategy"><option value="fixed">'+tr("固定时间 · 智能避让（默认）","Fixed times · Smart avoidance (default)")+'</option><option value="rolling">'+tr("连续窗口 · 智能滚动","Continuous windows · Smart rolling")+'</option></select><div class="field-hint"></div>';
  list.closest(".field-group")?.before(field);
  field.querySelector("select").value=draft.strategy;
  field.querySelector(".field-hint").textContent=draft.strategy==="rolling"?tr("设置首次启动时间。后续按每个账号真实的额度重置时间滚动。","Choose the first start time. Later runs follow each account's quota reset."):tr("请求前检查真实窗口；未到期时最多等待 10 分钟，否则跳过本次，不影响下次计划。","Check the real quota. Wait up to 10 minutes, otherwise skip this occurrence.");
  if(draft.strategy==="rolling"){
   list.querySelector(".add-time")?.remove();
   dialog.querySelector(".time-presets")?.remove();
   const label=list.closest(".field-group")?.querySelector(".label");
   if(label)label.textContent=tr("首次启动时间","First start time");
   dialog.querySelectorAll('[data-action="remove-time"]').forEach(el=>el.remove());
  }
  if(zone&&draft.strategy==="rolling"){
   const notice=document.createElement("div");notice.className="notice neutral";
   notice.textContent=tr("连续模式可能在凌晨运行；日期与时间仅用于首次启动。","Continuous mode may run overnight. The selected date and time are for initial startup only.");
   zone.closest(".field-group")?.before(notice);
  }
 };
 document.addEventListener("change",event=>{
  if(event.target.id!=="taskStrategy"||!draft)return;
  event.stopImmediatePropagation();readTaskForm();
  draft.strategy=event.target.value==="rolling"?"rolling":"fixed";
  if(draft.strategy==="rolling")draft.times=draft.times.slice(0,1);
  renderTaskDialog();
 },true);
 const oldStatusBadge=statusBadge;
 statusBadge=function(status,label){
  if(status==="waiting")return '<span class="badge orange">'+tr("等待窗口重置","Waiting for reset")+'</span>';
  return oldStatusBadge(status,label);
 };
 const originalRunDetail=renderRunDetail;
 renderRunDetail=function(run){
  // The dialog may be refreshed while a run is active. Replacing its innerHTML
  // destroys the scroller, so restore its position after the new DOM is ready.
  const dialog=document.querySelector("#detailDialog");
  const oldContent=dialog?.open?dialog.querySelector(".drawer-content"):null;
  const scrollTop=oldContent?.scrollTop||0;
  const scrollLeft=oldContent?.scrollLeft||0;
  originalRunDetail(run);
  const area=document.querySelector("#detailDialog .drawer-content");
  if(!area||!Array.isArray(run?.items))return;
  const panels=run.items.map(i=>{
   const translations={confirmed:tr("已确认开启新窗口","New window confirmed"),unconfirmed:tr("请求成功 · 窗口未确认","Request succeeded · window unconfirmed"),waiting:tr("等待窗口结束","Waiting for window reset"),existing:tr("已有有效窗口，未重复请求","Existing window · no request")};
   const info=translations[i.windowStatus];
   if(!info&&!i.dispatched&&!i.windowResetAt&&!i.nextWindowAt)return "";
   const fmt=n=>n?esc(new Date(n).toLocaleString()):"—";
   let rows=[[tr("计划时间","Scheduled"),i.scheduled],[tr("实际发起请求","Model request sent"),i.dispatched],[tr("执行前窗口到期","Previous reset"),i.windowResetAt],[tr("执行后窗口到期","Next reset"),i.nextWindowAt]];
   if(i.waitUntil)rows.push([tr("延迟执行时间","Deferred until"),i.waitUntil]);
   return '<div class="smart-window-detail"><strong translate="no">'+esc(i.authId||"")+'</strong><span class="smart-window-outcome">'+esc(info||tr("未获取窗口信息","Window data unavailable"))+'</span><dl>'+rows.map(([label,at])=>'<dt>'+label+'</dt><dd>'+fmt(at)+'</dd>').join("")+'</dl></div>';
  }).filter(Boolean).join("");
  if(panels)area.insertAdjacentHTML("beforeend",'<section class="smart-window-details"><h3>'+tr("智能窗口检查","Smart window verification")+'</h3>'+panels+'</section>');
  if(oldContent){area.scrollTop=scrollTop;area.scrollLeft=scrollLeft;}
 };
})();

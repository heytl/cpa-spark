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
 quickWarmupDialog.innerHTML=`<div class="modal-head"><div><h2 id="quickWarmupHeading">立即预热</h2><p class="small muted quick-target-name">${esc(q.name)}</p></div><button class="iconbtn" data-quick-close aria-label="关闭立即预热">${icon('x')}</button></div><div class="modal-body"><div class="quick-summary"><div><strong>${all.length}</strong><span>目标账号</span></div><div><strong>${eligible.length}</strong><span>可执行</span></div><div><strong>${skipped.length}</strong><span>跳过</span></div></div><div class="field-group"><label class="label" for="quickModel">模型</label><select class="field" id="quickModel" ${!context?.models.length?'disabled':''}>${context?.models.length?context.models.map(m=>`<option value="${esc(m.id)}" ${m.id===q.model?'selected':''}>${esc(m.id)}</option>`).join(''):`<option>${q.loading?'正在获取模型…':eligible.length?'没有可用模型':'没有可执行账号'}</option>`}</select></div><div class="field-group"><label class="label" for="quickEffort">推理强度</label><select class="field" id="quickEffort" ${!model?'disabled':''}>${choices.map(e=>`<option value="${esc(e.value)}" ${e.value===q.effort?'selected':''}>${esc(e.label)}</option>`).join('')}</select></div>${skipped.length?`<div class="notice neutral">${icon('info')}跳过 ${skipped.length} 个账号：${skipped.map(a=>esc(a.name)+'（'+esc(skipReason(a)||'已禁用')+'）').join('、')}</div>`:''}${q.error?`<p class="form-error" role="alert">${esc(q.error)}</p>`:''}<p class="small muted quick-note">仅执行一次，不新增定时任务。请求使用正常账号额度。</p></div><div class="modal-foot"><button class="btn" data-quick-close>取消</button><button class="btn primary" data-quick-confirm ${!ready||q.loading?'disabled':''}>${icon('bolt')}立即预热</button></div>`;
}
async function openQuickWarmup(kind,id){
 const g=kind==='group'?group(id):null,t=kind==='task'?task(id):null,a=kind==='account'?account(id):null;
 if(!g&&!t&&!a)return;
 const ids=[...new Set(g?g.members:t?targetIDs(t):[a.id])],provider=(g||t||a).provider;
 const seq=++quickWarmupSequence;
 quickWarmup={name:(g||t||a).name,ids,provider,model:'',effort:'',loading:true};renderQuickWarmup();if(!quickWarmupDialog.open)quickWarmupDialog.showModal();
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
function confirmQuickWarmup(button){
 const q=quickWarmup;if(!q||button.disabled)return;button.disabled=true;
 const eligible=q.ids.map(account).filter(a=>a&&a.provider===q.provider&&quickEligible(a));if(!eligible.length)return;
 const id='quick-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,7);
 previewRuns.unshift({id,taskId:id,name:'立即预热 · '+q.name,model:q.model,provider:q.provider,mode:'manual',started:Date.now(),finished:true,items:eligible.map((a,i)=>({id:id+'-'+i,authId:a.id,status:'success',message:'请求完成',latency:540+i*65,tokens:2,sources:[q.name],httpStatus:200}))});
 quickWarmupDialog.close();applySnapshot(previewSnapshot());toast(`预热完成：${eligible.length} 个账号成功`);
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

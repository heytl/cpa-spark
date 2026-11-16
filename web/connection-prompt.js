// A per-page credential fallback. No management keys are persisted by this component.
(() => {
 const zh={title:"连接 CPA",description:"无法自动读取 CPA 登录凭据，请手动输入管理密码。仅本次页面有效，不会保存。",password:"管理密码",connect:"连接",cancel:"取消",retry:"使用 CPA 已保存登录",empty:"请输入管理密码",failure:"认证失败，请检查管理密码。"};
 const en={title:"Connect CPA",description:"The remembered CPA credential cannot be read. Enter the management key for this page only; it will not be saved.",password:"Management key",connect:"Connect",cancel:"Cancel",retry:"Retry saved CPA login",empty:"Enter a management key",failure:"Authentication failed. Check the management key."};
 function words(){return typeof hostPreferences!=="undefined"&&String(hostPreferences.language).startsWith("zh")?zh:en}
 const dialog=document.createElement("dialog");dialog.id="sparkConnectionDialog";dialog.className="spark-connection-dialog";dialog.setAttribute("aria-label","Connect CPA");
 dialog.innerHTML='<form method="dialog" class="spark-connection-form"><h2 data-spark-title></h2><p data-spark-description></p><label><span data-spark-password-label></span><input type="password" autocomplete="off" maxlength="8192" data-spark-secret required></label><div class="spark-auth-error" role="alert" data-spark-error></div><div class="spark-connection-actions"><button type="button" class="btn" data-spark-cancel></button><button type="submit" class="btn primary" data-spark-connect></button></div><button type="button" class="spark-retry-login" data-spark-retry></button></form>';
 document.body.append(dialog);
 const fields={title:dialog.querySelector("[data-spark-title]"),description:dialog.querySelector("[data-spark-description]"),label:dialog.querySelector("[data-spark-password-label]"),secret:dialog.querySelector("[data-spark-secret]"),error:dialog.querySelector("[data-spark-error]"),submit:dialog.querySelector("[data-spark-connect]"),cancel:dialog.querySelector("[data-spark-cancel]"),retry:dialog.querySelector("[data-spark-retry]")};
 function copy(){const x=words();for(const [k,name] of Object.entries({title:"title",description:"description",label:"password",submit:"connect",cancel:"cancel",retry:"retry"}))fields[k].textContent=x[name]}
 function show(message=""){copy();fields.error.textContent=message||"";if(!dialog.open)dialog.showModal()}
 function hide(){fields.secret.value="";fields.error.textContent="";if(dialog.open)dialog.close()}
 fields.cancel.addEventListener("click",hide);
 fields.retry.addEventListener("click",()=>{CPAPluginSession.clearManualConnection(window);hide();void connectLive()});
 dialog.querySelector("form").addEventListener("submit",async event=>{
  event.preventDefault();const value=fields.secret.value;fields.secret.value="";
  if(!CPAPluginSession.setManualConnection(window,value)){fields.error.textContent=words().empty;return}
  fields.submit.disabled=true;
  try {await connectLive();if(connected)hide();else show(words().failure)}
  finally {fields.submit.disabled=false}
 });
 globalThis.CPAConnectionPrompt=Object.freeze({show,hide});
})();

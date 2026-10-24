from pathlib import Path
import argparse,json,re,base64
root=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser();parser.add_argument('--output',type=Path,default=root/'dist/CPA-Spark-Static-Preview.html');args=parser.parse_args();out=args.output.resolve()
html=(root/'web/index.html').read_text();css=(root/'web/style.css').read_text();js=(root/'web/app.js').read_text()
js=js.split('// CPA_PRODUCT_UI_START')[0]
# Keep the preview typography identical to the shipped application.
# The preview is self-contained: no stored credentials, backend, storage writes or network.
start=js.index('/* CPA WebUI storage interoperability.')
end=js.index('// Production bridge:',start)
js=js[:start]+"const CPAPluginSession=Object.freeze({savedConnection:()=>({managementKey:'static-preview-only'}),resourcePrefix:()=>'',loginURL:()=> '#settings'});\n"+js[end:]
a=js.index('async function authenticatedAPI(url,data){');b=js.index('function currentConfig()',a)
js=js[:a]+"async function authenticatedAPI(url,data){return previewAPI(url,data)}\n"+js[b:]
js=js.replace('function resolveHostPreferences(', 'function previewResolveHostPreferences(',1)
js+='\nfunction resolveHostPreferences(env){if(env.parent!==env)return previewResolveHostPreferences(env);return {language:\'zh-CN\',mode:\'light\',theme:\'white\',source:\'fallback\'}}\n'
# Production templates already include the approved presentation.
html=html.replace('href="./logo.svg"', 'href="data:image/svg+xml;base64,'+base64.b64encode((root/'logo.svg').read_bytes()).decode('ascii')+'"')
html=re.sub(r'<title>.*?</title>','<title>CPA Spark · 星火 — 静态效果预览</title>',html,count=1)
html=re.sub(r'<meta name="description"[^>]*>','<meta name="description" content="CPA Spark（星火）— 轻量预热，按时就绪。">',html,count=1)
html=re.sub(r'<div class="plugin-title">[\s\S]*?</div><p class="plugin-description">[\s\S]*?</p>', '<div class="plugin-title">CPA Spark <span class="brand-cn">星火</span></div><p class="plugin-description">轻量预热，按时就绪。</p>',html,count=1)

bootstrap=r'''
const previewAccounts=[
{id:'pc4',name:'Codex 已禁用账号',email:'disabled@example.com',provider:'codex',status:'disabled',disabled:true},
{id:'pc1',name:'Codex 主账号',email:'main@example.com',provider:'codex',plan:'ChatGPT Plus',status:'active'},
{id:'pc2',name:'Codex 备用账号',email:'backup@example.com',provider:'codex',plan:'ChatGPT Pro',status:'active'},
{id:'pc3',name:'Codex 工作账号',email:'work@example.com',provider:'codex',status:'active'},
{id:'pg1',name:'Gemini 主账号',email:'gemini@example.com',provider:'antigravity',plan:'Google AI Pro',status:'active'},
{id:'pg2',name:'Gemini 备用账号',email:'flash@example.com',provider:'antigravity',plan:'Individual',status:'active'}];
const previewConfig={revision:1,settings:{timezone:'Asia/Shanghai',concurrency:2,retention:30,late:120,paused:false},overrides:{},groups:[{id:'gc',name:'Codex 工作组',provider:'codex',members:['pc4','pc1','pc2','pc3'],description:'',color:'orange'},{id:'gg',name:'Gemini 轻量组',provider:'antigravity',members:['pg1','pg2'],description:'',color:'green'}],tasks:[
{id:'pt1',name:'主力账号唤醒',provider:'codex',accountIds:['pc1'],groupIds:[],excludes:[],times:['06:00','11:00','16:00'],days:[0,1,2,3,4,5,6],timezone:'Asia/Shanghai',model:'gpt-6-luna',effort:'none',enabled:true,revision:1},
{id:'pt2',name:'工作日账号预热',provider:'codex',accountIds:[],groupIds:['gc'],excludes:[],times:['08:00','13:00','18:00'],days:[1,2,3,4,5],timezone:'Asia/Shanghai',model:'gpt-6-luna',effort:'none',enabled:true,revision:1},
{id:'pt3',name:'Gemini 轻量计划',provider:'antigravity',accountIds:[],groupIds:['gg'],excludes:[],times:['09:00','14:00','19:00'],days:[1,2,3,4,5],timezone:'Asia/Shanghai',model:'gemini-3-flash',effort:'budget:0',enabled:false,revision:1}]};
const previewModels={codex:[{id:'gpt-6-luna',thinking:{levels:['none','low','medium','high']}},{id:'gpt-6-sol',thinking:{levels:['low','medium','high']}}],antigravity:[{id:'gemini-3-flash',thinking:{min:128,max:8192,zero_allowed:true}},{id:'gemini-3-pro',thinking:{levels:['low','high']}}]};
const previewRuns=previewConfig.tasks.slice(0,2).map((t,i)=>({id:'pr'+i,taskId:t.id,name:t.name,model:t.model,provider:t.provider,mode:'scheduled',started:Date.now()-(i+1)*8*60000,finished:true,items:(i?['pc1','pc2','pc3']:['pc1']).map((id,n)=>({id:'pi'+i+n,authId:id,status:'success',message:'请求完成',latency:620+n*80,tokens:2,sources:[t.name],httpStatus:200}))}));
function previewSnapshot(){return {version:'静态预览',config:structuredClone(previewConfig),runs:structuredClone(previewRuns),now:Date.now(),active:0,lastError:''}}
async function previewAPI(url,data){
 const path=String(url).split('?')[0];
 if(path.endsWith('/state'))return previewSnapshot();
 if(path.endsWith('/accounts'))return {accounts:structuredClone(previewAccounts)};
 if(path.endsWith('/auth-files/models')){const name=new URL(String(url),'https://preview.invalid').searchParams.get('name');const provider=previewAccounts.find(a=>a.id===name)?.provider||'codex';return {models:previewModels[provider].map(({id})=>({id}))}}
 if(path.includes('/model-definitions/'))return {models:structuredClone(previewModels[path.split('/').at(-1)]||[])};
 if(path.endsWith('/history'))return previewHistoryPage(previewRuns,previewAccounts,data);
 if(path.endsWith('/history-detail'))return structuredClone(previewRuns.find(r=>r.id===data.id));
 if(path.endsWith('/account-timeline'))return {runs:structuredClone(previewRuns)};
 if(path.endsWith('/preview'))return {times:occurrences(data.task,5).map(at=>new Date(at).toISOString())};
 if(path.endsWith('/config')){Object.assign(previewConfig,structuredClone(data),{revision:previewConfig.revision+1});return previewSnapshot()}
 if(path.endsWith('/delete-history')){
  const accountMode=data.mode==='account';
  const done=i=>!['queued','running'].includes(i.executionStatus||i.status);
  const selected=accountMode?[...new Set(previewRuns.flatMap(r=>r.items.filter(i=>done(i)&&(data.all||data.executionIds?.includes(i.id)||data.filter&&previewHistoryPage(previewRuns,previewAccounts,{mode:'account',filter:data.filter,pageSize:100000}).rows.some(r=>r.id===i.id))).map(i=>i.id)))]:previewRuns.filter(r=>r.finished&&(data.all||data.ids?.includes(r.id)||data.filter&&(!data.filter.taskId||r.taskId===data.filter.taskId)&&(!data.filter.query||[r.name,r.model,r.id].join(' ').toLowerCase().includes(data.filter.query.toLowerCase()))&&(!data.filter.date||dateKey(r.started)===data.filter.date)&&(!data.filter.status||data.filter.status==='all'||runStatus(r)===data.filter.status))).map(r=>r.id);
  if(!data.preview){const ids=new Set(selected);if(accountMode)previewRuns.forEach(r=>r.items=r.items.filter(i=>!ids.has(i.id)));previewRuns.splice(0,previewRuns.length,...previewRuns.filter(r=>accountMode?r.items.length:!ids.has(r.id)))}
  return {count:selected.length};
 }
 if(path.endsWith('/cancel'))return {ok:true};
 throw Error('静态预览，不执行服务端操作');
}
'''
script=bootstrap+'\n'+(Path(__file__).parent/'preview-history.js').read_text()+'\n'+js+'\n'+(root/'web/window-timeline.js').read_text()+'\n'+(Path(__file__).parent/'preview-ui.js').read_text()+'\n'+(Path(__file__).parent/'preview-actions.js').read_text()+'\n'+(Path(__file__).parent/'preview-controls.js').read_text()+'\n'+(root/'web/history-data.js').read_text()+'\n'+(root/'web/history-ui.js').read_text()
html=html.replace('<link rel="stylesheet" href="./style.css">','<style>'+css+'</style>')
html=html.replace('<script src="./app.js" defer></script>','<script>'+script.replace('</script','<\\/script')+'</script>')
out.parent.mkdir(parents=True,exist_ok=True);out.write_text(html)
# Keep the preview source available alongside the builder, without touching live assets.
(Path(__file__).parent/'CPA-Spark-Static-Preview.html').write_text(html)
print(json.dumps({'path':str(out),'bytes':out.stat().st_size,'self_contained':True},ensure_ascii=False))

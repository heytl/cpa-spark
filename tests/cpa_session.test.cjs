const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const path=require('node:path');
const root=path.join(__dirname,'..');
const sessionSource=fs.readFileSync(path.join(root,'web/cpa-session.js'),'utf8');
const bridgeSource=fs.readFileSync(path.join(root,'web/live-bridge.js'),'utf8').split('startRun=async')[0];
function fixture({mount='',encrypted=true,remember=true,base}={}){
 const location=new URL('https://cpa.example'+mount+'/v0/resource/plugins/cpa-spark/status');
 const navigator={userAgent:'session-test'};
 const values=new Map([['isLoggedIn','true']]);
 function store(name,value){let raw=JSON.stringify(value);if(encrypted){const bytes=Buffer.from(raw),mask=Buffer.from(`cli-proxy-api-webui::secure-storage|${location.host}|${navigator.userAgent}`);for(let i=0;i<bytes.length;i++)bytes[i]^=mask[i%mask.length];raw='enc::v1::'+bytes.toString('base64')}values.set(name,raw)}
 store('cli-proxy-auth',{state:{apiBase:base||location.origin+mount,managementKey:'test-only-密钥',rememberPassword:remember},version:0});
 const elements=new Map();
 const element=id=>{if(!elements.has(id))elements.set(id,{hidden:false,textContent:'',addEventListener(){}});return elements.get(id)};
 const document={body:{dataset:{},prepend(){}},createElement:()=>element('banner'),getElementById:element,querySelector:()=>null,querySelectorAll:()=>[],addEventListener(){}};
 const win={location,navigator,document,localStorage:{getItem:k=>values.get(k)??null,setItem(){throw Error('plugin must not persist credentials')}},addEventListener(){}};
 const context=vm.createContext({window:win,location,navigator,document,URL,TextEncoder,TextDecoder,atob,console,state:{accounts:[],tasks:[],groups:[],runs:[]},activeRuns:new Map(),selected:new Set(),settingsDraft:null,detailType:'',render(){},toast(){},clone:structuredClone,$:()=>null});
 vm.runInContext(sessionSource,context);
 return {context,values,store,elements,session:()=>context.CPAPluginSession.savedConnection(win)};
}
for(const encrypted of [true,false])test('official saved session '+(encrypted?'obfuscated':'plaintext'),()=>{
 const f=fixture({encrypted,mount:'/proxy'});assert.equal(f.session().managementKey,'test-only-密钥');
 assert.equal(f.context.CPAPluginSession.loginURL(f.context.window),'https://cpa.example/proxy/management.html#/login');
});
test('missing saved password and logout do not reuse credentials',()=>{
 const f=fixture({remember:false});assert.equal(f.session(),null);
 f.store('cli-proxy-auth',{state:{apiBase:'https://cpa.example',managementKey:'old',rememberPassword:true}});
 f.values.delete('isLoggedIn');assert.equal(f.session(),null);
});
test('legacy encrypted fields remain compatible',()=>{
 const f=fixture();f.values.delete('cli-proxy-auth');f.store('apiBase','https://cpa.example/v8/management');f.store('managementKey','legacy-test');assert.equal(f.session().managementKey,'legacy-test');
});
for(const base of ['https://other.example','http://cpa.example','https://cpa.example/other','https://user@cpa.example','https://cpa.example/?redirect=elsewhere'])test('reject unrelated connection '+base,()=>assert.equal(fixture({base}).session(),null));
test('unknown or malformed storage fails closed',()=>{
 const f=fixture();for(const value of ['enc::v2::abc','enc::v1::%%%%','{broken']){f.values.set('cli-proxy-auth',value);assert.equal(f.session(),null)}
});
function bridge(f,fetch){f.context.fetch=fetch;vm.runInContext(bridgeSource,f.context);return code=>vm.runInContext(code,f.context)}
const snapshot={version:'test',now:'2026-10-21T00:00:00Z',config:{revision:0,tasks:[],groups:[],settings:{},overrides:{}},runs:[]};
const response=(body,status=200)=>({ok:status<400,status,json:async()=>body});
test('automatically connects using CPA session and clears data on logout',async()=>{
 const f=fixture();const requests=[];const run=bridge(f,async(url,options)=>{requests.push({url,options});return response(url.endsWith('/accounts')?{accounts:[{id:'test-account',provider:'codex'}]}:snapshot)});
 await run('connectLive()');assert.equal(run('connected'),true);assert.equal(f.context.state.accounts[0].id,'test-account');
 assert.deepEqual(requests.map(r=>r.options.headers.Authorization),['Bearer test-only-密钥','Bearer test-only-密钥']);
 assert.equal(f.elements.has('banner'),false);
 f.values.delete('isLoggedIn');run('synchronizeLogin()');assert.equal(run('connected'),false);assert.equal(f.context.state.accounts.length,0);assert.equal(run('managementKey'),'');
 assert.equal(requests.length,2);
});
test('without a saved login no protected request is made',async()=>{
 const f=fixture({remember:false});const run=bridge(f,()=>{throw Error('unexpected network call')});await run('connectLive()');assert.equal(run('connected'),false);assert.match(run('connectionStatus'),/CPA/);
});
test('late responses after logout cannot restore account data',async()=>{
 const f=fixture();const pending=[];const run=bridge(f,url=>new Promise(resolve=>pending.push(()=>resolve(response(url.endsWith('/accounts')?{accounts:[{id:'stale',provider:'codex'}]}:snapshot)))));
 const connecting=run('connectLive()');f.values.delete('isLoggedIn');run('synchronizeLogin()');pending.forEach(resolve=>resolve());await connecting;assert.equal(run('connected'),false);assert.equal(f.context.state.accounts.length,0);
});
test('401 disconnects and does not expose a key prompt',async()=>{
 const f=fixture();const run=bridge(f,async()=>response({error:'unauthorized'},401));await run('connectLive()');assert.equal(run('connected'),false);assert.equal(run('managementKey'),'');assert.match(run('connectionStatus'),/登录已失效/);
});

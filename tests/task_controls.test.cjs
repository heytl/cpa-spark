const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs'),path=require('node:path');
const context=vm.createContext({});vm.runInContext(fs.readFileSync(path.join(__dirname,'../web/task-controls.js'),'utf8')+';globalThis.rules=CPATaskControls',context);const rules=context.rules;
const ids=list=>list.map(id=>({id}));
test('offers the union of selected account models, ignoring empty catalogs',()=>assert.deepEqual(Array.from(rules.modelOptions([ids(['luna','sol','image-model']),ids(['luna','image-model']),[]]),m=>m.id),['luna','sol']));
test('prefers available Luna and Gemini Flash models',()=>{
 assert.equal(rules.preferredModel(ids(['gpt-6-sol','gpt-6-luna']),'codex'),'gpt-6-luna');
 assert.equal(rules.preferredModel(ids(['gemini-3-pro','gemini-3-flash']),'antigravity'),'gemini-3-flash');
 assert.equal(rules.preferredModel(ids(['gemini-3-flash','gemini-3-flash-lite']),'antigravity'),'gemini-3-flash-lite');
 assert.equal(rules.preferredModel([],'codex'),'');
});
test('orders only declared reasoning levels by lowest consumption',()=>{
 assert.deepEqual(Array.from(rules.efforts({thinking:{levels:['high','low','minimal']}}),e=>({value:e.value,label:e.label})),[{value:'minimal',label:'最低'},{value:'low',label:'低'},{value:'high',label:'高'}]);
 assert.equal(rules.efforts({thinking:{levels:['low','none','high']}})[0].value,'none');
});
test('native budget chooses zero when supported and minimum otherwise',()=>{
 assert.equal(rules.efforts({thinking:{min:128,max:8192,zero_allowed:true}})[0].value,'budget:0');
 assert.equal(rules.efforts({thinking:{min:128,max:8192}})[0].value,'budget:128');
 assert.equal(rules.efforts({})[0].value,'');
});
test('new time slots remain unique across midnight',()=>{
 assert.equal(rules.nextTime(['23:30']),'00:30');
 assert.equal(rules.nextTime(['00:30','23:30']),'00:31');
});
test('bundled zone catalog covers IANA aliases and Shanghai default',()=>{
 const zones=JSON.parse(fs.readFileSync(path.join(__dirname,'../web/timezones.json'),'utf8'));
 assert(zones.length>500);for(const zone of ['Asia/Shanghai','UTC','Europe/Paris','America/St_Johns','Asia/Kathmandu','Pacific/Chatham'])assert(zones.includes(zone),zone);
});
test('five-hour preset starts with the first time and stops at midnight',()=>{
 assert.deepEqual(Array.from(rules.fiveHourTimes('07:35')),['07:35','12:35','17:35','22:35']);
 assert.deepEqual(Array.from(rules.fiveHourTimes('23:59')),['23:59']);
 assert.deepEqual(Array.from(rules.fiveHourTimes('00:00')),['00:00','05:00','10:00','15:00','20:00']);
 assert.deepEqual(Array.from(rules.fiveHourTimes('invalid')),[]);
});

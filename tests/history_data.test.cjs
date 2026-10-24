const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
const context=vm.createContext({});vm.runInContext(fs.readFileSync('web/history-data.js','utf8')+';globalThis.rules=CPAHistory',context);
test('account history deduplicates merged requests while retaining task sources and actual status',()=>{
 const shared={id:'execution-1',authId:'account-1',started:2000,status:'skipped',executionStatus:'success'};
 const runs=[{id:'run-1',taskId:'task-1',name:'First',started:1000,model:'m',items:[shared]},{id:'run-2',taskId:'task-2',name:'Second',started:1000,model:'m',items:[{...shared,status:'success'},{id:'execution-2',authId:'account-2',status:'running'}]}];
 const rows=context.rules.accounts(runs,[{id:'account-1',name:'Saved name'}]);
 assert.equal(rows.length,2);assert.equal(rows[0].id,'execution-1');assert.equal(rows[0].started,2000);assert.equal(rows[0].status,'success');assert.equal(rows[0].runs.length,2);assert.equal(rows[0].account.name,'Saved name');
});

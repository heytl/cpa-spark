const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs');
test('account timeline clips both rolling windows and deduplicates merged executions',()=>{
 const now=1000000000,day=86400000;
 const tasks=[{id:'t',enabled:true,times:['06:00'],model:'m'},{id:'paused',enabled:false,times:[],model:'m'}];
 const item=(id,at,status='success')=>({id,authId:'a',status,finished:at});
 const runs=[{id:'r',name:'Saved name',model:'m',started:now-2*day,items:[item('recent',now-1000),item('old',now-day-1),item('edge',now-day),item('future',now+1),item('queued',now-100,'queued'),{...item('other',now-10),authId:'b'}]},{id:'merged',started:now-500,items:[item('recent',now-1000)]}];
 const c=vm.createContext({state:{tasks,runs},targetIDs:()=>['a'],occurrences:()=>[now+1,now+day,now+day+1],profile:()=>':m'});vm.runInContext(fs.readFileSync('web/account-timeline.js','utf8'),c);const result=vm.runInContext(`accountTimelineData('a',${now})`,c);
 assert.deepEqual(Array.from(result.upcoming,e=>e.at),[now+1,now+day]);assert.equal(result.past.length,1);assert.equal(result.past[0].name,'Saved name');assert.equal(result.past[0].at,now-1000);
});

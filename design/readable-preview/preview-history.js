// Offline fixtures only; production pagination is performed by SQLite.
function previewHistoryPage(runs,accounts,query){
 const accountMode=query.mode!=='task',filter=query.filter||{},items=accountMode?CPAHistory.accounts(runs,accounts):runs.map(r=>({...r,status:runStatus(r),accountCount:r.items.length}));
 const all=items.filter(r=>!['queued','running'].includes(r.status)).length;
 let rows=items.filter(row=>{
  const sources=accountMode?row.runs:[row],text=accountMode?[row.account?.name,row.account?.email,row.authId,row.model,row.id,...sources.flatMap(r=>[r.name,r.id])]:[row.name,row.model,row.id];
  return (!filter.taskId||sources.some(r=>r.taskId===filter.taskId))&&(!filter.query||text.join(' ').toLowerCase().includes(filter.query.toLowerCase()))&&(!filter.status||filter.status==='all'||row.status===filter.status)&&(!filter.date||dateKey(row.started,filter.timezone)===filter.date);
 }).sort((a,b)=>b.started-a.started||b.id.localeCompare(a.id));
 const total=rows.length,deletable=rows.filter(r=>!['queued','running'].includes(r.status)).length,size=query.pageSize||50,pages=Math.max(1,Math.ceil(total/size)),page=Math.min(pages,query.page||1);
 return {rows:rows.slice((page-1)*size,page*size),total,deletable,allDeletable:all,page,pages,pageSize:size};
}

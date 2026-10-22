// Complete-message catalogs with stable IDs, named parameters and English fallback.
// The source adapter supports existing templates; new UI should use t(id, params)
// or data-i18n="id". It never substitutes words inside a sentence.
const translatedText=new WeakMap(),translatedAttributes=new WeakMap();
const i18nNormalize=s=>String(s).replace(/\s+/g,' ').trim();
const i18nExact=new Map(),i18nPatterns=[];
function escapePattern(s){return s.replace(/[.*+?^${}()|[\]\\]/g,'\\$&')}
for(const[id,message]of Object.entries(CPA_I18N_CATALOG)){
 for(const value of new Set([message.source,...Object.values(message).filter(v=>typeof v==='string'),...Object.values(message.plural||{}).filter(v=>typeof v==='object').flatMap(forms=>Object.values(forms))])){
  const source=i18nNormalize(value),slots=[...source.matchAll(/\{(\w+)\}/g)];
  if(!slots.length){if(!i18nExact.has(source))i18nExact.set(source,{id,params:{}});continue}
  let at=0,pattern='^';for(const slot of slots){pattern+=escapePattern(source.slice(at,slot.index)).replace(/ /g,'\\s*')+'(.*?)';at=slot.index+slot[0].length}pattern+=escapePattern(source.slice(at)).replace(/ /g,'\\s*')+'$';
  i18nPatterns.push({id,slots:slots.map(s=>s[1]),regex:new RegExp(pattern,'u'),weight:source.replace(/\{\w+\}/g,'').length});
 }
}
i18nPatterns.sort((a,b)=>b.weight-a.weight);
function protectedDisplayValues(){return new Set([...state.accounts.flatMap(a=>[a.name,a.email,a.name?.split(' · ')[0]]),...state.groups.flatMap(g=>[g.name,g.description]),...state.tasks.map(t=>t.name),...(state.runs||[]).map(r=>r.name)].filter(Boolean))}
function resolveMessage(source){const normalized=i18nNormalize(source),exact=i18nExact.get(normalized);if(exact)return exact;for(const p of i18nPatterns){const match=p.regex.exec(normalized);if(match)return {id:p.id,params:Object.fromEntries(p.slots.map((slot,i)=>[slot,match[i+1]]))}}return null}
function t(id,params={},language=hostPreferences.language){
 const message=CPA_I18N_CATALOG[id];if(!message)throw Error('Unknown translation key: '+id);
 const resolved=language==='zh-CN'?'source':message[language]?language:'en';
 let text=message[resolved];
 if(message.plural?.[resolved]){const form=new Intl.PluralRules(resolved).select(Number(params[message.plural.parameter]));text=message.plural[resolved][form]||text}
 return text.replace(/\{(\w+)\}/g,(_,name)=>String(params[name]??''));
}
function uiText(value,language=hostPreferences.language,depth=0){
 if(value==null)return '';const source=String(value),message=resolveMessage(source);if(!message)return source;
 const protectedValues=typeof state==='undefined'?new Set():protectedDisplayValues();
 const params=Object.fromEntries(Object.entries(message.params).map(([key,value])=>[key,depth<4&&value!==i18nNormalize(source)&&!protectedValues.has(value)?uiText(value,language,depth+1):value]));
 return source.match(/^\s*/)[0]+t(message.id,params,language)+source.match(/\s*$/)[0];
}
function localizeUI(root){
 if(!root)return;
 const skip=el=>el?.closest?.('script,style,textarea,[translate="no"],[data-i18n-skip],.task-name,.name-link,.quick-target-name,.st-target-name,.group-desc,.group-card h2,.selection-item h3,.account-hero h2,.upcoming-title,.account-event p,.target-label .ellipsis');
 const translateNode=node=>{if(skip(node.parentElement)||node.parentElement?.closest('[data-i18n]'))return;let record=translatedText.get(node);if(!record||node.nodeValue!==record.last)record={source:node.nodeValue,last:node.nodeValue};const value=uiText(record.source);if(node.nodeValue!==value)node.nodeValue=value;record.last=value;translatedText.set(node,record)};
 if(root.nodeType===3){translateNode(root);return}if(root.nodeType!==1||skip(root))return;
 for(const el of [root,...root.querySelectorAll('[data-i18n]')])if(el.dataset.i18n){const value=t(el.dataset.i18n,JSON.parse(el.dataset.i18nParams||'{}'));if(el.textContent!==value)el.textContent=value}
 const walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);let node;while(node=walker.nextNode())translateNode(node);
 for(const el of [root,...root.querySelectorAll('[aria-label],[title],[placeholder]')]){
  if(el.closest('script,style,[translate="no"],[data-i18n-skip]'))continue;
  const records=translatedAttributes.get(el)||{};
  for(const attr of ['aria-label','title','placeholder']){if(!el.hasAttribute(attr))continue;const current=el.getAttribute(attr);let record=records[attr];if(!record||current!==record.last)record={source:current,last:current};const value=uiText(record.source);if(current!==value)el.setAttribute(attr,value);record.last=value;records[attr]=record}translatedAttributes.set(el,records);
 }
}

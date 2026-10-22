// A changed/new source message requires an explicit catalog entry before packaging.
const fs=require('node:fs'),assert=require('node:assert/strict'),{inventory}=require('./i18n_inventory.cjs');
const catalog=JSON.parse(fs.readFileSync('web/locales/messages.json','utf8'));
const sources=new Set(Object.values(catalog).map(m=>m.source));
const slots=s=>[...s.matchAll(/\{(\w+)\}/g)].map(m=>m[1]).sort();
for(const[id,m]of Object.entries(catalog)){
 assert(m.source&&m.en,'Missing source/English translation: '+id);
 for(const[language,text]of Object.entries(m)){if(language==='plural'){for(const forms of Object.values(text).filter(v=>typeof v==='object'))for(const form of Object.values(forms))assert.deepEqual(slots(form),slots(m.source),'Plural parameter mismatch: '+id);continue}assert(!text.includes('[REVIEW]'),'Unreviewed message: '+id);if(language!=='source')assert.deepEqual(slots(text),slots(m.source),'Placeholder mismatch: '+id+'/'+language)}
}
let js=fs.readFileSync('web/app.js','utf8').split('// CPA_I18N_CATALOG_END\n').at(-1);
const acorn=require('acorn'),walk=require('acorn-walk');
const checkMarkup=text=>{for(const m of text.matchAll(/data-i18n="([^"]+)"/g))assert(catalog[m[1]],'Unknown translation key: '+m[1])};
walk.simple(acorn.parse(js,{ecmaVersion:'latest'}),{CallExpression(n){if(n.callee.type==='Identifier'&&n.callee.name==='t'&&n.arguments[0]?.type==='Literal')assert(catalog[n.arguments[0].value],'Unknown translation key: '+n.arguments[0].value)},Literal(n){if(typeof n.value==='string')checkMarkup(n.value)},TemplateElement(n){checkMarkup(n.value.cooked||'')}});

const missing=inventory(js,fs.readFileSync('web/index.html','utf8')).filter(s=>!sources.has(s));
if(missing.length){console.error('Add complete messages to web/locales/messages.json:\n'+missing.join('\n'));process.exit(1)}
console.log('PASS: i18n source coverage and translation parameters');

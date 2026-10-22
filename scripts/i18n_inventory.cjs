// Extract whole UI messages, including template placeholders, rather than substrings.
const fs=require('node:fs'),acorn=require('acorn'),walk=require('acorn-walk'),{JSDOM}=require('jsdom');
function inventory(js,html=''){
 js=js.split('// CPA_I18N_CATALOG_END\n').at(-1);
 const found=new Set(),add=s=>{s=s.replace(/\s+/g,' ').trim();if(/[\u3400-\u9fff]/u.test(s))found.add(s)};
 const document=new JSDOM('').window.document;
 function markup(s){const tpl=document.createElement('template');tpl.innerHTML=s;const walker=document.createTreeWalker(tpl.content,4);let n;while(n=walker.nextNode())if(!n.parentElement?.closest('script,style,[translate="no"],[data-i18n-skip]'))add(n.nodeValue);for(const el of tpl.content.querySelectorAll('[title],[aria-label],[placeholder]'))for(const a of ['title','aria-label','placeholder'])if(el.hasAttribute(a))add(el.getAttribute(a));}
 js=js.replace(/const UI_MESSAGES=[\s\S]*?;\nconst translatedText/, 'const translatedText');
 const tree=acorn.parse(js,{ecmaVersion:'latest',sourceType:'script'});
 walk.simple(tree,{Literal(n){if(typeof n.value==='string'&&/[\u3400-\u9fff]/u.test(n.value)){if(/<\/?[a-z][^>]*>/i.test(n.value))markup(n.value);else add(n.value)}},TemplateLiteral(n){let s=n.quasis.map((q,i)=>q.value.cooked+(i<n.expressions.length?'{v'+i+'}':'')).join('');if(/<\/?[a-z][^>]*>/i.test(s))markup(s);else add(s)}});
 if(html)markup(html);document.defaultView.close();return [...found].sort();
}
module.exports={inventory};
if(require.main===module)console.log(JSON.stringify(inventory(fs.readFileSync('web/app.js','utf8'),fs.readFileSync('web/index.html','utf8')),null,2));

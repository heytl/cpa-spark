const {test}=require('node:test');
const assert=require('node:assert/strict');
const vm=require('node:vm');
const fs=require('node:fs');
const source=fs.readFileSync('design/prototype.html','utf8').split('/* CPA_HOST_PREFERENCES_START */')[1].split('/* CPA_I18N_RUNTIME */')[0];
function fixture(){
 const values=new Map(),attrs=new Map(),tokens=new Map();
 const root={style:{getPropertyValue:k=>values.get(k)||'',setProperty:(k,v)=>values.set(k,v),removeProperty:k=>values.delete(k)}};
 const parentRoot={getAttribute:k=>attrs.get(k)||null};
 const storage={getItem:()=>null};
 const env={localStorage:storage,parent:{document:{documentElement:parentRoot},localStorage:storage,getComputedStyle:()=>({getPropertyValue:k=>tokens.get(k)||''})},navigator:{language:'en'},matchMedia:()=>({matches:false})};
 const context=vm.createContext({window:env,document:{documentElement:root},CSS:{supports:(_,v)=>!v.includes('url(')}});
 vm.runInContext(source,context);return {values,attrs,tokens,run:s=>vm.runInContext(s,context)};
}
test('CPA effective theme wins over local system preference and updates without reloading',()=>{
 const f=fixture();for(const theme of ['dark','white','light']){f.attrs.set('data-theme',theme);assert.equal(f.run('resolveHostPreferences(window).theme'),theme)}
 f.attrs.delete('data-theme');assert.equal(f.run('resolveHostPreferences(window).theme'),'light');
});
test('imports current CPA palette and removes stale tokens when host stops supplying them',()=>{
 const f=fixture();f.tokens.set('--bg-secondary','#151412');f.tokens.set('--text-primary','#f6f4f1');f.run('synchronizeHostPalette()');assert.equal(f.values.get('--cpa-bg'),'#151412');
 f.tokens.set('--bg-secondary','#ffffff');f.run('synchronizeHostPalette()');assert.equal(f.values.get('--cpa-bg'),'#ffffff');
 f.tokens.clear();f.run('synchronizeHostPalette()');assert.equal(f.values.size,0);
});
test('unavailable cross-origin host falls back without throwing',()=>{
 const f=fixture();f.run("Object.defineProperty(window.parent,'document',{get(){throw Error('cross-origin')}})");assert.doesNotThrow(()=>f.run('synchronizeHostPalette()'));assert.equal(f.run('resolveHostPreferences(window).source'),'fallback');
});
test('standalone automatic mode resolves current system theme on every synchronization',()=>{
 const f=fixture();f.run("window.parent=window;window.localStorage.getItem=k=>k==='cli-proxy-theme'?JSON.stringify({state:{theme:'auto'}}):null;window.matchMedia=()=>({matches:true})");
 assert.equal(f.run('resolveHostPreferences(window).theme'),'dark');
 f.run('window.matchMedia=()=>({matches:false})');assert.equal(f.run('resolveHostPreferences(window).theme'),'white');
});

from pathlib import Path
import re,sys,json,subprocess
source=Path(sys.argv[1]) if len(sys.argv)>1 else Path('design/prototype.html')
notices = [('CLIProxyAPI native ABI', 'cliproxyapi-LICENSE.txt'),
           ('CPA Management Center storage compatibility', 'cpa-webui-LICENSE.txt'),
           ('Lucide icons', 'lucide-LICENSE.txt')]
Path('web/third-party-notices.txt').write_text('\n\n'.join(
    title + '\n\n' + Path('docs/third-party', name).read_text().strip()
    for title, name in notices) + '\n', encoding='utf-8', newline='\n')
release_version=re.search(r'const Version = "([^"]+)"',Path('internal/warmup/types.go').read_text())[1]
s=source.read_text()
messages=json.loads(Path('web/locales/messages.json').read_text())
s=s.replace('/* CPA_I18N_RUNTIME */',Path('web/i18n.js').read_text())
s=re.sub(r'<meta name="description" content="[^"]*">', '<meta name="description" content="CPA Spark（星火）：为 CLIProxyAPI 提供单账号与自定义分组的独立预热调度、计划预览和执行追踪。">', s, count=1)
css=re.search(r'<style>([\s\S]*?)</style>',s).group(1)
js=re.search(r'<script>([\s\S]*?)</script>',s).group(1)
s=re.sub(r'<style>[\s\S]*?</style>','<link rel="stylesheet" href="./style.css">',s,count=1)
s=re.sub(r'<script>[\s\S]*?</script>','<script src="./app.js" defer></script>',s,count=1)
# Strip demo bootstrap, storage and all simulation entry points.
js=re.sub(r'function seed\(\)\{[\s\S]*?\n\}',"function seed(){return {schema:1,revision:0,accounts:[],groups:[],tasks:[],runs:[],overrides:{},settings:{timezone:'Asia/Shanghai',concurrency:4,retention:30,late:120,paused:false}}}",js,count=1)
js=re.sub(r"let state;try\{[^\n]*",'let state=seed();',js,count=1)
for name in ['persist','startRun','pumpSimulations','refreshSimulations','recoverSimulatedRuns','saveTask','saveGroup']:
 js=re.sub(r'^function '+name+r'\([^\n]*', 'function '+name+'(){}',js,flags=re.M)
js=js.replace('initializeHostPreferences();persist();render();localizeUI(document.body);','initializeHostPreferences();render();localizeUI(document.body);')
js=js.replace("model:'gpt-6-luna'","model:''").replace("draft.model=MODELS[el.value][0]","draft.model=''")
old='<select class="field" id="taskModel">${MODELS[t.provider].map(v=>`<option value="${v}" ${t.model===v?\'selected\':\'\'}>${v}</option>`).join(\'\')}</select>'
assert old in js
js=js.replace(old,'<input class="field" id="taskModel" value="${esc(t.model)}" placeholder="CPA model ID" autocomplete="off">')
js=js.replace("if(!id&&draft.provider==='antigravity')draft.model=MODELS.antigravity[0];",'')
js=js.replace('原型使用示例模型。正式插件将读取所选账号的可用模型。','请填写 CPA 中已验证可用的模型 ID；保存后仍由上游校验模型权限。')
js=js.replace("if(step>=2&&!targets(draft).length)","if(step>=2&&!draft.model.trim())return'请填写模型 ID';if(step>=2&&!targets(draft).length)")
js=js.replace('items=today.flatMap(r=>r.items)', 'items=[...new Map(today.flatMap(r=>r.items).map(i=>[i.id,i])).values()]')
# Explicit, consented live execution dialog.
js=js.replace('此操作只生成模拟结果，不会调用真实 CPA 或模型账号。','此操作将通过 CPA 向指定账号发送真实请求，会消耗正常额度。')
js=js.replace('本批次使用虚构数据。成功仅代表模拟请求成功，不代表真实额度或窗口变化。','本批次由 CPA 服务端执行。请求成功不代表额度窗口已开启或重置。')
js=js.replace('${(i.latency/1000).toFixed(2)}s · ${i.tokens} tokens · 模拟值','${(i.latency/1000).toFixed(2)}s · ${i.tokens==null?\'usage unknown\':i.tokens+\' tokens\'}')
js=js.replace('function settingsPage()', 'function prototypeSettingsPage()') if 'function settingsPage()' in js else js
for old,new in [('交互原型 v0.3','v'+release_version),('模拟数据 · 未连接真实 CPA','CPA 服务端 · 正式请求'),('模拟执行','立即执行'),('今日模拟成功','今日请求成功'),('正在逐账号模拟执行','正在执行账号请求'),('开始立即执行','发送真实请求'),('模拟请求不会访问真实账号','请求通过 CPA 指定账号执行'),('演示数据保存在当前浏览器','配置保存在 CPA 插件 SQLite'),('浏览器关闭后仍应由 CPA 服务端调度。此原型仅展示计划，不会后台发请求。','定时任务由 CPA 服务端调度，关闭浏览器后仍继续执行。'),('计划已启用 · 模拟环境','服务端定时计划已启用')]:
 s=s.replace(old,new);js=js.replace(old,new)
js=js.replace("draft.times.push('12:00')", "draft.times.push(CPATaskControls.nextTime(draft.times))")
js=js.replace("$('#helpIcon').innerHTML=icon('help');", '')
js=js.replace("t.effort||'low'", "t.effort||''")
# Remove demo reset controls. The runtime intercepts all mutations before the original dispatcher.
js=js.replace("case 'reset-demo':", "case 'disabled-reset-demo':")
# Raise the old 9–12px supporting text without changing the production assets.
def readable(match):return match[1]+str(max(13,int(match[2])))+'px'
css=css.replace('font:14px/1.5','font:16px/1.65')
css=re.sub(r'(font-size\s*:\s*)(\d+)px',readable,css)
js=re.sub(r'(font-size\s*:\s*)(\d+)px',readable,js)
# Remove the card and technical footnotes from the HTML-generating template itself.
js=re.sub(r'<section class="tip-panel">[\s\S]*?</section>','',js,count=1)
js=js.replace('5 小时窗口时间轴','窗口可视化')
js=re.sub(r'<div class="notice info" style="margin-bottom:20px">\$\{icon\(\'info\'\)\}这里展示模拟请求结果。[\s\S]*?</div>','',js,count=1)
js=js.replace('<span>所有结果均为演示数据</span>','')
for before,after in [('手动模拟','手动预热'),('定时演示','定时预热')]:js=js.replace(before,after)
a=js.index('function renderAccounts()');b=js.index('\nfunction renderHistory',a)
accounts=js[a:b]
accounts=accounts.replace("state.accounts.filter(a=>(", "state.accounts.filter(a=>(!filters.accountStatus||filters.accountStatus==='all'||(filters.accountStatus==='enabled')===previewAccountEnabled(a))&&(",1)
accounts=accounts.replace("includes(filters.accountQuery.toLowerCase())),gList=", "includes(filters.accountQuery.toLowerCase())).sort(previewEnabledFirst),gList=",1)
accounts=accounts.replace('<span class="grow"></span><span class="tiny muted nowrap">', '${!isGroups?previewStatusFilter():\'\'}<span class="grow"></span><span class="tiny muted nowrap">',1)
accounts=accounts.replace('<th>预热开关</th>', '<th>预热开关</th><th style="text-align:left">操作</th>')
accounts=accounts.replace('return `<tr><td style="padding-left:20px">','return `<tr data-account-id="${esc(a.id)}"><td style="padding-left:20px">',1)
accounts=accounts.replace('data-action="pause-account" data-id="${a.id}"></button></td></tr>', 'data-action="pause-account" data-id="${a.id}"></button></td><td style="text-align:left"><button class="btn compact" data-quick-warmup="account" data-id="${esc(a.id)}" ${!previewAccountEnabled(a)?\'disabled title="账号已禁用"\':\'\'}>${icon(\'bolt\')}立即预热</button></td></tr>')
accounts=accounts.replace('<div class="group-card-footer"><span>${tasks.length} 条关联任务</span>', '<div class="group-quick-action"><button class="btn compact" data-quick-warmup="group" data-id="${esc(g.id)}" ${!g.members.some(id=>previewAccountEnabled(account(id)))?\'disabled\':\'\'}>${icon(\'bolt\')}立即预热</button></div><div class="group-card-footer"><span>${tasks.length} 条关联任务</span>')
js=js[:a]+accounts+js[b:]
js=js.replace('as=state.accounts.filter(a=>a.provider===t.provider);return', 'as=state.accounts.filter(a=>a.provider===t.provider).sort(previewEnabledFirst);return')
js=js.replace('list=state.accounts.filter(a=>a.provider===g.provider);', 'list=state.accounts.filter(a=>a.provider===g.provider).sort(previewEnabledFirst);')
js=js.replace('服务端定时计划已启用','调度已启用')
js=js.replace("download('cpa-spark-demo-history.json',{notice:'All results are simulated.',runs:state.runs})", "download('cpa-spark-history.json',{timezone:state.settings.timezone,runs:state.runs.map(r=>({...r,startedText:previewTimestamp(r.started)}))})")
js=js.replace('共 ${list.length} 个任务 · 时区以任务设置为准','共 ${list.length} 个任务')
# Format record timestamps independently of compact schedule clocks.
js=js.replace('${date(r.started)} ${time(r.started)}','${previewTimestamp(r.started)}')
js=js.replace("state.settings.timezone==='Asia/Shanghai'?'上海时间':esc(state.settings.timezone)","esc(previewZoneLabel(state.settings.timezone))")
js=js.replace("t.timezone==='Asia/Shanghai'?'上海':esc(t.timezone)","esc(previewZoneLabel(t.timezone))")
js=js.replace('esc(state.settings.timezone)', 'esc(previewZoneLabel(state.settings.timezone))')
js=js.replace('esc(t.timezone)', 'esc(previewZoneLabel(t.timezone))')
js=js.replace('esc(zone)', 'esc(previewZoneLabel(zone))')
s=re.sub(r'<div class="plugin-title">[\s\S]*?</div><p class="plugin-description">[\s\S]*?</p>', '<div class="plugin-title">CPA Spark <span class="brand-cn">星火</span></div><p class="plugin-description">轻量预热，按时就绪。</p>', s, count=1)
Path('web/style.css').write_text(css+'\n'+Path('web/task-editor.css').read_text()+'\n'+Path('web/product-ui.css').read_text()+'\n'+Path('web/window-timeline.css').read_text()+'\n'+Path('web/host-theme.css').read_text()+'\nbody[data-disconnected="true"] #main{pointer-events:none;opacity:.45}\n',newline='\n')
s=re.sub(r'<div class="plugin-tools">[\s\S]*?</div>', '', s, count=1)
s=s.replace('class="plugin-version"','class="plugin-version" translate="no"')
s=re.sub(r'^[ \t]+$','',s,flags=re.M)
Path('web/index.html').write_text(s,newline='\n')
Path('web/app.js').write_text('const CPA_SPARK_VERSION='+json.dumps(release_version)+';\nconst CPA_I18N_CATALOG='+json.dumps(messages,ensure_ascii=False,separators=(',',':'))+';\n// CPA_I18N_CATALOG_END\nconst CPA_TIME_ZONES='+Path('web/timezones.json').read_text()+';\n'+js+'\n'+Path('web/cpa-session.js').read_text()+'\n'+Path('web/live-bridge.js').read_text()+'\n'+Path('web/task-controls.js').read_text()+'\n'+Path('web/task-editor.js').read_text()+'\n'+Path('web/account-timeline.js').read_text()+'\n'+Path('web/brand.js').read_text()+'\n// CPA_PRODUCT_UI_START\n'+Path('web/window-timeline.js').read_text()+'\n'+Path('web/product-ui.js').read_text(),newline='\n')
subprocess.run(['node','scripts/check_i18n.cjs'],check=True)
print('Built real-mode assets without demo accounts or simulated execution')

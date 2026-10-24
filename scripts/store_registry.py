"""Generate a registry from the actual repository identity; never guess an owner."""
import argparse,json,re
p=argparse.ArgumentParser();p.add_argument('--repository',required=True);p.add_argument('--author',required=True);p.add_argument('--logo');a=p.parse_args()
if not re.fullmatch(r'https://github\.com/[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+',a.repository):p.error('repository must be https://github.com/owner/repo')
if not a.author.strip() or any(x in a.author for x in ['\n','\r']):p.error('author must be nonempty')
entry={'id':'cpa-spark','name':'CPA Spark · 星火','description':'Account-pinned warmup and scheduling for CLIProxyAPI, with independent account/group tasks, schedule previews, execution history, and CPA appearance inheritance.','author':a.author,'repository':a.repository,'homepage':a.repository,'tags':['Scheduler','Management','Codex','Antigravity']}
a.logo = a.logo or a.repository.replace('https://github.com/', 'https://raw.githubusercontent.com/', 1) + '/main/logo.svg'
if a.logo:
 if not a.logo.startswith('https://'):p.error('logo requires an HTTPS URL')
 entry['logo']=a.logo
print(json.dumps({'schema_version':1,'plugins':[entry]},ensure_ascii=False,indent=2))

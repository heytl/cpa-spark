"""Native platform build + ABI load gate. Run on each target's matching OS/CPU."""
import argparse,os,platform,re,shutil,subprocess,sys
from pathlib import Path
from release_artifacts import ROOT,TARGETS,source_version
p=argparse.ArgumentParser();p.add_argument('--target',required=True,choices=TARGETS);a=p.parse_args()
system={'Linux':'linux','Darwin':'darwin','Windows':'windows'}.get(platform.system());arch={'x86_64':'amd64','AMD64':'amd64','aarch64':'arm64','arm64':'arm64'}.get(platform.machine())
if f'{system}_{arch}'!=a.target:raise SystemExit('Native build must run on its matching OS and architecture')
subprocess.run([sys.executable,str(ROOT/'scripts/build_web.py')],cwd=ROOT,check=True)
version=source_version();tag=os.environ.get('CPA_RELEASE_TAG','')
if tag and tag!='v'+version:raise SystemExit('Git tag must match source Version')
go=os.environ.get('GO','go');out=ROOT/'dist/native'/a.target;out.mkdir(parents=True,exist_ok=True);library=out/f'cpa-spark.{TARGETS[a.target]}'
flags=['-s','-w']
repo=os.environ.get('GITHUB_REPOSITORY','');author=os.environ.get('GITHUB_REPOSITORY_OWNER','')
if repo:
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+',repo):raise SystemExit('Invalid GitHub repository')
    flags+=['-X',f'cpa-spark/internal/warmup.Repository=https://github.com/{repo}']
if author:
    if not re.fullmatch(r'[A-Za-z0-9_-]+',author):raise SystemExit('Invalid GitHub owner')
    flags+=['-X',f'cpa-spark/internal/warmup.Author={author}']
env={**os.environ,'CGO_ENABLED':'1','GOOS':system,'GOARCH':arch}
if system=='windows':env['CGO_LDFLAGS']=(env.get('CGO_LDFLAGS','')+' -static-libgcc').strip()
subprocess.run([go,'build','-buildvcs=false','-buildmode=c-shared','-ldflags',' '.join(flags),'-o',str(library),'./cmd/plugin'],cwd=ROOT,env=env,check=True)
smoke=[sys.executable,str(ROOT/'tests/native_smoke.py'),str(library),'--expected-version',version,'--report',str(out/'abi.json')]
if repo:smoke+=['--expected-repository','https://github.com/'+repo]
if author:smoke+=['--expected-author',author]
subprocess.run(smoke,cwd=ROOT,check=True)
download=ROOT/'dist/downloads'/a.target/f'cpa-spark-v{version}.{TARGETS[a.target]}'
download.parent.mkdir(parents=True,exist_ok=True)
shutil.copy2(library,download)
print(download)

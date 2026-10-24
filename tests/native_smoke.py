"""Exercise the built C ABI using a fake CPA host. Never contacts real model accounts."""
import argparse,base64,ctypes as C,hashlib,json,pathlib,platform,re,sys,tempfile,time,threading
parser=argparse.ArgumentParser();parser.add_argument("library");parser.add_argument("--expected-version");parser.add_argument("--report");parser.add_argument("--expected-repository");parser.add_argument("--expected-author");args=parser.parse_args()
expected_version=args.expected_version or re.search(r'const Version = "([^"]+)"',(pathlib.Path(__file__).resolve().parents[1]/"internal/warmup/types.go").read_text())[1]
lib=C.CDLL(str(pathlib.Path(args.library).resolve()))
class Buffer(C.Structure):_fields_=[('ptr',C.c_void_p),('len',C.c_size_t)]
HOSTCALL=C.CFUNCTYPE(C.c_int,C.c_void_p,C.c_char_p,C.c_void_p,C.c_size_t,C.POINTER(Buffer))
FREE=C.CFUNCTYPE(None,C.c_void_p,C.c_size_t)
PLUGCALL=C.CFUNCTYPE(C.c_int,C.c_char_p,C.c_void_p,C.c_size_t,C.POINTER(Buffer))
STOP=C.CFUNCTYPE(None)
class Host(C.Structure):_fields_=[('abi',C.c_uint32),('ctx',C.c_void_p),('call',HOSTCALL),('free',FREE)]
class Plugin(C.Structure):_fields_=[('abi',C.c_uint32),('call',PLUGCALL),('free',FREE),('stop',STOP)]
buffers={};model_calls=[];guard=threading.Lock()
@HOSTCALL
def host_call(ctx,method,p,n,out):
 try:
  request=json.loads(C.string_at(p,n));name=method.decode()
  if name=='host.auth.list':result={'files':[{'id':'abi-account','name':'ABI TEST','provider':'codex','status':'active'}]}
  elif name=='host.model.execute':
   assert request['auth_id']=='abi-account' and request['forced_provider']=='codex'
   with guard:model_calls.append(request)
   result={'status_code':200,'body':base64.b64encode(b'{"choices":[{"message":{"content":"OK"}}],"usage":{"total_tokens":4}}').decode()}
  else:raise AssertionError(name)
  raw=json.dumps({'ok':True,'result':result}).encode();buf=C.create_string_buffer(raw)
  with guard:buffers[C.addressof(buf)]=buf
  out.contents.ptr=C.addressof(buf);out.contents.len=len(raw);return 0
 except BaseException as error:print('HOST FAILURE',error);return 1
@FREE
def host_free(p,n):
 with guard:buffers.pop(p,None)
host=Host(1,None,host_call,host_free);plugin=Plugin();lib.cliproxy_plugin_init.argtypes=[C.POINTER(Host),C.POINTER(Plugin)]
assert lib.cliproxy_plugin_init(None,C.byref(plugin))!=0
assert lib.cliproxy_plugin_init(C.byref(host),None)!=0
host.abi=0;assert lib.cliproxy_plugin_init(C.byref(host),C.byref(plugin))!=0
host.abi=1;assert lib.cliproxy_plugin_init(C.byref(host),C.byref(plugin))==0
for method,pointer,size in [(None,None,0),(b'plugin.register',None,1),(b'plugin.register',None,2*1024*1024+1)]:
 out=Buffer();assert plugin.call(method,pointer,size,C.byref(out))!=0;assert not out.ptr and out.len==0
assert plugin.call(b'plugin.register',None,0,None)!=0

def rpc(method,request):
 raw=json.dumps(request).encode();buf=C.create_string_buffer(raw);out=Buffer();code=plugin.call(method.encode(),buf,len(raw),C.byref(out));assert code==0,(method,code)
 try:body=json.loads(C.string_at(out.ptr,out.len))
 finally:plugin.free(out.ptr,out.len)
 assert body['ok'],body
 return body['result']
def endpoint(method,route,data=None):
 request={'Method':method,'Path':'/v0/management/plugins/cpa-spark/'+route,'Headers':{},'Body':base64.b64encode(json.dumps(data).encode()).decode() if data is not None else ''}
 response=rpc('management.handle',request);body=json.loads(base64.b64decode(response['Body']));assert response['StatusCode']<300,body;return body
with tempfile.TemporaryDirectory() as directory:
 try:
  registration=rpc('plugin.register',{'schema_version':6,'config_yaml':base64.b64encode(('enabled: true\nstate_dir: '+directory+'\n').encode()).decode()});assert registration['capabilities']=={'management_api':True}
  assert registration['metadata']['Name']=='CPA Spark'
  assert registration['metadata']['Version']==expected_version
  logo_bytes=(pathlib.Path(__file__).resolve().parents[1]/'logo.svg').read_bytes()
  sidebar_bytes=(pathlib.Path(__file__).resolve().parents[1]/'sidebar-icon.svg').read_bytes()
  assert registration['metadata']['Logo']=='data:image/svg+xml;base64,'+base64.b64encode(sidebar_bytes).decode()
  assert b'<rect' not in sidebar_bytes and b'#808080' in sidebar_bytes
  if args.expected_repository:assert registration['metadata']['GitHubRepository']==args.expected_repository
  if args.expected_author:assert registration['metadata']['Author']==args.expected_author
  # Match CPA's validPlugin metadata gate, including local builds without CI -X flags.
  for field in ('Name','Version','Author','GitHubRepository'):
   assert isinstance(registration['metadata'].get(field),str) and registration['metadata'][field].strip(), ('CPA rejects empty registration metadata',field,registration)
  routes=rpc('management.register',{});assert len(routes['routes'])==11
  for endpoint_name in ('history','history-detail','account-timeline'):
   assert any(r['Method']=='POST' and r['Path']=='/plugins/cpa-spark/'+endpoint_name for r in routes['routes'])
  resource=rpc('management.handle',{'Method':'GET','Path':'/v0/resource/plugins/cpa-spark/status'});assert resource['StatusCode']==200
  script=rpc('management.handle',{'Method':'GET','Path':'/v0/resource/plugins/cpa-spark/app.js'});assert script['StatusCode']==200
  assert base64.b64decode(resource['Body'])==(pathlib.Path(__file__).resolve().parents[1]/'web/index.html').read_bytes()
  assert base64.b64decode(script['Body'])==(pathlib.Path(__file__).resolve().parents[1]/'web/app.js').read_bytes()
  stylesheet=rpc('management.handle',{'Method':'GET','Path':'/v0/resource/plugins/cpa-spark/style.css'});assert stylesheet['StatusCode']==200
  assert base64.b64decode(stylesheet['Body'])==(pathlib.Path(__file__).resolve().parents[1]/'web/style.css').read_bytes()
  logo=rpc('management.handle',{'Method':'GET','Path':'/v0/resource/plugins/cpa-spark/logo.svg'})
  assert logo['StatusCode']==200 and logo['Headers']['Content-Type']==['image/svg+xml']
  assert base64.b64decode(logo['Body'])==logo_bytes
  sidebar=rpc('management.handle',{'Method':'GET','Path':'/v0/resource/plugins/cpa-spark/sidebar-icon.svg'})
  assert sidebar['StatusCode']==200 and sidebar['Headers']['Content-Type']==['image/svg+xml']
  assert base64.b64decode(sidebar['Body'])==sidebar_bytes
  notices=rpc('management.handle',{'Method':'GET','Path':'/v0/resource/plugins/cpa-spark/third-party-notices.txt'})
  assert notices['StatusCode']==200 and notices['Headers']['Content-Type']==['text/plain; charset=utf-8']
  notice_text=base64.b64decode(notices['Body']).decode()
  assert 'Router-For.ME' in notice_text and 'Lucide' in notice_text and 'MIT License' in notice_text
  script_text=base64.b64decode(script['Body']).decode();assert 'CPAPluginSession.savedConnection' in script_text and 'liveManagementKey' not in script_text
  config=endpoint('GET','state')['config'];assert not config['tasks'];accounts=endpoint('GET','accounts')['accounts'];assert accounts[0]['id']=='abi-account'
  config['tasks']=[{'id':'abi-task','name':'ABI task','provider':'codex','accountIds':['abi-account'],'groupIds':[],'excludes':[],'days':[1,2,3,4,5],'times':['06:00'],'timezone':'Asia/Shanghai','model':'abi-model','effort':'low','enabled':False,'revision':1}]
  saved=endpoint('POST','config',config);assert saved['config']['revision']==1
  preview=endpoint('POST','preview',{'task':config['tasks'][0]});assert len(preview['times'])==5
  run=endpoint('POST','run',{'taskId':'abi-task','key':'native-smoke-1'});assert run==endpoint('POST','run',{'taskId':'abi-task','key':'native-smoke-1'})
  until=time.monotonic()+8
  while time.monotonic()<until:
   state=endpoint('GET','state')
   if state['runs'] and state['runs'][0]['finished']:break
   time.sleep(.05)
  assert state['runs'][0]['summary'] and not state['runs'][0]['items'],state
  detail=endpoint('POST','history-detail',{'id':state['runs'][0]['id']})
  assert detail['items'][0]['status']=='success',detail
  page=endpoint('POST','history',{'mode':'account','pageSize':20})
  assert page['total']==1 and len(page['rows'])==1 and page['rows'][0]['status']=='success',page
  assert len(model_calls)==1
  count=endpoint('POST','delete-history',{'ids':[run['runId']],'preview':True});assert count['count']==1
  once=endpoint('POST','run-once',{'task':config['tasks'][0],'key':'native-once-test'})
  assert len(endpoint('GET','state')['config']['tasks'])==1
  until=time.monotonic()+8
  while time.monotonic()<until:
   state=endpoint('GET','state')
   if all(r['finished'] for r in state['runs']):break
   time.sleep(.05)
  assert len(model_calls)==2
  assert endpoint('POST','delete-history',{'all':True})['count']==2
  assert not endpoint('GET','state')['runs']
  rpc('plugin.quiesce',{});print('PASS: native ABI registration, embedded assets, account binding, SQLite config, five previews, idempotent run, host model callback, and quiesce')
 finally:plugin.stop()
assert not buffers,'host buffers leaked'

if args.report:
 target={'Linux':'linux','Darwin':'darwin','Windows':'windows'}[platform.system()]+'_'+{'x86_64':'amd64','AMD64':'amd64','aarch64':'arm64','arm64':'arm64'}[platform.machine()]
 pathlib.Path(args.report).write_text(json.dumps({'passed':True,'id':'cpa-spark','name':registration['metadata']['Name'],'version':registration['metadata']['Version'],'target':target,'repository':registration['metadata']['GitHubRepository'],'author':registration['metadata']['Author'],'sha256':hashlib.sha256(pathlib.Path(args.library).read_bytes()).hexdigest()},indent=2)+'\n')

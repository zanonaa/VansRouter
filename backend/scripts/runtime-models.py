import subprocess, os, time, urllib.request, urllib.error, json, http.cookiejar
processes=[]
try:
 for name,port,command in [('next',20371,['/node/bin/node','/modules/next/dist/bin/next','dev','--webpack','--hostname','127.0.0.1','--port','20371']),('go',20372,['/scratch/tmp/vansrouter-models'])]:
  env=dict(os.environ,DATA_DIR='/scratch/models-'+name,PORT=str(port),INITIAL_PASSWORD='123456')
  processes.append(subprocess.Popen(command,cwd='/scratch/project',env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL))
 clients=[urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())) for _ in range(2)]
 def call(i,method,path,payload=None):
  request=urllib.request.Request(f'http://127.0.0.1:{20371+i}'+path,data=None if payload is None else json.dumps(payload).encode(),method=method,headers={'Content-Type':'application/json'})
  try:response=clients[i].open(request,timeout=60)
  except urllib.error.HTTPError as error:response=error
  raw=response.read()
  return response.status,json.loads(raw) if raw else None
 for i in range(2):
  for _ in range(90):
   try:call(i,'GET','/api/ready');break
   except (OSError,urllib.error.URLError):time.sleep(1)
  else:raise AssertionError('startup timeout')
 def normalize(value):
  if isinstance(value,list):
   return [normalize(v) for v in value]
  if isinstance(value,dict):return {k:normalize(v) for k,v in value.items() if k not in ('id','createdAt','updatedAt')}
  return value
 count=0
 def compare(method,path,payload=None,skip_body=False):
  global count
  values=[call(i,method,path,payload) for i in range(2)]
  assert values[0][0]==values[1][0],(method,path,'status',values[0][0],values[1][0])
  if not skip_body:
   a,b=normalize(values[0][1]),normalize(values[1][1])
   if a!=b:
    # Modelos dinâmicos (fetchers) e caps podem divergir entre instâncias;
    # registrar claramente em vez de falhar silenciosamente.
    fa,fb=json.dumps(a)[:800],json.dumps(b)[:800]
    raise AssertionError(f'{method} {path} body differs:\nNEXT={fa}\nGO={fb}')
  count+=1
  return values
 compare('GET','/api/models/disabled')
 compare('POST','/api/auth/login',{'password':'123456'})
 compare('POST','/api/models/disabled',{})
 compare('POST','/api/models/disabled',{'providerAlias':'agentrouter','ids':['claude-opus-4-6','other']})
 compare('GET','/api/models/disabled')
 compare('GET','/api/models/disabled?providerAlias=agentrouter')
 compare('GET','/api/models/disabled?providerAlias=missing')
 compare('DELETE','/api/models/disabled')
 compare('DELETE','/api/models/disabled?providerAlias=agentrouter&id=claude-opus-4-6')
 compare('GET','/api/models/disabled?providerAlias=agentrouter')
 values=compare('GET','/api/models')
 target=None
 for m in values[0][1]['models']:
  if m.get('fullModel') and m.get('provider')=='agentrouter':target=m['fullModel'];break
 assert target,'no agentrouter model'
 compare('PUT','/api/models',{})
 compare('PUT','/api/models',{'model':target,'alias':'runtime-alias'})
 values=compare('GET','/api/models')
 found=any(m.get('alias')=='runtime-alias' for m in values[0][1]['models'])
 assert found,'alias not applied in Next listing'
 found=any(m.get('alias')=='runtime-alias' for m in values[1][1]['models'])
 assert found,'alias not applied in Go listing'
 compare('PUT','/api/models',{'model':'other/model','alias':'runtime-alias'})
 compare('DELETE','/api/models/disabled?providerAlias=agentrouter')
 print(f'PASS runtime models: {count} requests Next/Go (alias em {target})')
finally:
 for p in processes:p.terminate()
 for p in processes:
  try:p.wait(timeout=10)
  except subprocess.TimeoutExpired:p.kill();p.wait()

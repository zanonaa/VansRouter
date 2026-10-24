"""Comparação Next/Go em namespace isolado preparado, sem rede externa."""
import subprocess, os, time, urllib.request, urllib.error, json, http.cookiejar
processes=[]
try:
 for name,port,command in [('next',20351,['/node/bin/node','/modules/next/dist/bin/next','dev','--webpack','--hostname','127.0.0.1','--port','20351']),('go',20352,['/scratch/tmp/vansrouter-routing'])]:
  env=dict(os.environ,DATA_DIR='/scratch/combos-'+name,PORT=str(port),INITIAL_PASSWORD='123456')
  processes.append(subprocess.Popen(command,cwd='/scratch/project',env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL))
 clients=[urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())) for _ in range(2)]
 def call(i,method,path,payload=None):
  request=urllib.request.Request(f'http://127.0.0.1:{20351+i}'+path,data=None if payload is None else json.dumps(payload).encode(),method=method,headers={'Content-Type':'application/json'})
  try:response=clients[i].open(request,timeout=40)
  except urllib.error.HTTPError as error:response=error
  raw=response.read()
  return response.status,json.loads(raw) if raw else None
 for i in range(2):
  for _ in range(60):
   try:call(i,'GET','/api/ready');break
   except (OSError,urllib.error.URLError):time.sleep(1)
  else:raise AssertionError('startup timeout')
 def normalize(value):
  if isinstance(value,list):return [normalize(v) for v in value]
  if isinstance(value,dict):return {k:normalize(v) for k,v in value.items() if k not in ('id','createdAt','updatedAt')}
  return value
 count=0
 def compare(method,paths,payload=None):
  global count
  if isinstance(paths,str):paths=[paths,paths]
  values=[call(i,method,paths[i],payload) for i in range(2)]
  assert values[0][0]==values[1][0] and normalize(values[0][1])==normalize(values[1][1]),(method,paths,values)
  count+=1
  return values
 compare('GET','/api/combos')
 compare('POST','/api/auth/login',{'password':'123456'})
 compare('POST','/api/combos',{})
 compare('POST','/api/combos',{'name':'invalid space'})
 compare('POST','/api/combos',{'name':'x','context_length':0})
 values=compare('POST','/api/combos',{'name':'runtime-combo','models':['openai/model'],'context_length':'4096'})
 paths=['/api/combos/'+v[1]['id'] for v in values]
 compare('POST','/api/combos',{'name':'runtime-combo'})
 compare('GET',paths)
 compare('PUT',paths,{'name':'renamed','models':[],'context_length':None})
 compare('GET','/api/combos')
 compare('HEAD',paths)
 compare('OPTIONS',paths)
 compare('DELETE',paths)
 compare('GET',paths)
 print(f'PASS runtime combos: {count} requests Next/Go')
finally:
 for p in processes:p.terminate()
 for p in processes:
  try:p.wait(timeout=10)
  except subprocess.TimeoutExpired:p.kill();p.wait()

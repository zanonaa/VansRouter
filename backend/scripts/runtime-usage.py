import subprocess, os, time, urllib.request, urllib.error, json, http.cookiejar
processes=[]
try:
 for name,port,command in [('next',20381,['/node/bin/node','/modules/next/dist/bin/next','dev','--webpack','--hostname','127.0.0.1','--port','20381']),('go',20382,['/scratch/tmp/vansrouter-usage'])]:
  env=dict(os.environ,DATA_DIR='/scratch/usage-'+name,PORT=str(port),INITIAL_PASSWORD='123456')
  processes.append(subprocess.Popen(command,cwd='/scratch/project',env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL))
 clients=[urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())) for _ in range(2)]
 def call(i,method,path,payload=None):
  request=urllib.request.Request(f'http://127.0.0.1:{20381+i}'+path,data=None if payload is None else json.dumps(payload).encode(),method=method,headers={'Content-Type':'application/json'})
  try:response=clients[i].open(request,timeout=60)
  except urllib.error.HTTPError as error:response=error
  raw=response.read()
  return response.status,json.loads(raw) if raw else None
 for i in range(2):
  for _ in range(90):
   try:call(i,'GET','/api/ready');break
   except (OSError,urllib.error.URLError):time.sleep(1)
  else:raise AssertionError('startup timeout')
 def normalize(v):
  if isinstance(v,list):return [normalize(x) for x in v]
  if isinstance(v,dict):return {k:normalize(x) for k,x in v.items() if k not in ('id','createdAt','updatedAt','timestamp')}
  return v
 count=0
 def compare(method,path,payload=None):
  global count
  values=[call(i,method,path,payload) for i in range(2)]
  assert values[0][0]==values[1][0],(method,path,values[0][0],values[1][0])
  assert normalize(values[0][1])==normalize(values[1][1]),(method,path,json.dumps(values[0][1])[:400],json.dumps(values[1][1])[:400])
  count+=1
  return values
 compare('GET','/api/usage/history')
 compare('POST','/api/auth/login',{'password':'123456'})
 compare('GET','/api/usage/history')
 compare('GET','/api/usage/chart')
 compare('GET','/api/usage/chart?period=today')
 compare('GET','/api/usage/chart?period=all')
 compare('GET','/api/usage/chart?period=invalid')
 compare('GET','/api/usage/providers')
 print(f'PASS runtime usage: {count} requests Next/Go (vazio nos dois)')
finally:
 for p in processes:p.terminate()
 for p in processes:
  try:p.wait(timeout=10)
  except subprocess.TimeoutExpired:p.kill();p.wait()

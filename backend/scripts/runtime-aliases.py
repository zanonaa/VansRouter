"""Comparação Next/Go de aliases e modelos customizados; sandbox preparado."""
import subprocess, os, time, urllib.request, urllib.error, json, http.cookiejar
processes=[]
try:
 for name,port,command in [('next',20361,['/node/bin/node','/modules/next/dist/bin/next','dev','--webpack','--hostname','127.0.0.1','--port','20361']),('go',20362,['/scratch/tmp/vansrouter-aliases'])]:
  env=dict(os.environ,DATA_DIR='/scratch/aliases-'+name,PORT=str(port),INITIAL_PASSWORD='123456')
  processes.append(subprocess.Popen(command,cwd='/scratch/project',env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL))
 clients=[urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())) for _ in range(2)]
 def call(i,method,path,payload=None):
  request=urllib.request.Request(f'http://127.0.0.1:{20361+i}'+path,data=None if payload is None else json.dumps(payload).encode(),method=method,headers={'Content-Type':'application/json'})
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
 def compare(method,path,payload=None):
  global count
  values=[call(i,method,path,payload) for i in range(2)]
  assert values[0][0]==values[1][0] and normalize(values[0][1])==normalize(values[1][1]),(method,path,values)
  count+=1
  return values
 compare('GET','/api/models/alias')
 compare('POST','/api/auth/login',{'password':'123456'})
 compare('PUT','/api/models/alias',{})
 compare('PUT','/api/models/alias',{'model':'openai/gpt'})
 compare('PUT','/api/models/alias',{'model':'openai/gpt','alias':'fast'})
 compare('GET','/api/models/alias')
 compare('PUT','/api/models/alias',{'model':'openai/gpt-2','alias':'fast'})
 compare('GET','/api/models/alias')
 compare('DELETE','/api/models/alias')
 compare('DELETE','/api/models/alias?alias=fast')
 compare('GET','/api/models/custom')
 compare('POST','/api/models/custom',{})
 compare('POST','/api/models/custom',{'providerAlias':'oa','id':'m','caps':{'vision':True,'bad':'x'},'transport':'gemini-live'})
 compare('GET','/api/models/custom')
 compare('POST','/api/models/custom',{'providerAlias':'oa','id':'m','name':'Updated','caps':{'reasoning':True}})
 compare('GET','/api/models/custom')
 compare('POST','/api/models/custom',{'models':[{'providerAlias':'oa','id':'a'},{'providerAlias':'oa','id':'a'},{'id':'skip'},{'providerAlias':'oa','id':'b','name':'B'}]})
 compare('POST','/api/models/custom',{'providerAlias':'oa','id':'stt','type':'stt','transport':'gemini-live'})
 compare('GET','/api/models/custom')
 compare('DELETE','/api/models/custom')
 compare('DELETE','/api/models/custom?providerAlias=oa&id=a')
 compare('DELETE','/api/models/custom?providerAlias=oa&id=m&type=llm')
 compare('GET','/api/models/custom')
 print(f'PASS runtime aliases: {count} requests Next/Go')
finally:
 for p in processes:p.terminate()
 for p in processes:
  try:p.wait(timeout=10)
  except subprocess.TimeoutExpired:p.kill();p.wait()

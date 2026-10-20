"""Executar dentro do sandbox preparado: /scratch/project, /node, /modules.
Compara Next real e Go no mesmo namespace isolado com DATA_DIRs separados.
"""
import subprocess,time,urllib.request,urllib.error,json,http.cookiejar,os
processes=[]
logs=[]
try:
 for name,port,command in [('next',20341,['/node/bin/node','/modules/next/dist/bin/next','dev','--webpack','--hostname','127.0.0.1','--port','20341']),('go',20342,['/scratch/tmp/vansrouter-migration'])]:
  env=dict(os.environ,DATA_DIR='/scratch/runtime-'+name,PORT=str(port),INITIAL_PASSWORD='123456')
  log=open('/scratch/runtime-'+name+'.log','w');logs.append(log)
  processes.append(subprocess.Popen(command,cwd='/scratch/project',env=env,stdout=log,stderr=log))
 clients=[urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())) for _ in range(2)]
 def call(i,method,path,payload=None):
  req=urllib.request.Request(f'http://127.0.0.1:{20341+i}'+path,data=None if payload is None else json.dumps(payload).encode(),method=method,headers={'Content-Type':'application/json'})
  try:r=clients[i].open(req,timeout=40)
  except urllib.error.HTTPError as e:r=e
  raw=r.read();body=json.loads(raw) if raw else None
  return r.status,r.headers,body
 for i in range(2):
  for attempt in range(60):
   try:call(i,'GET','/api/ready');break
   except (OSError,urllib.error.URLError):time.sleep(1)
  else:raise AssertionError('server startup timeout')
 def comparable(body):
  if isinstance(body,list):return [comparable(v) for v in body]
  if isinstance(body,dict):return {k:comparable(v) for k,v in body.items() if k not in ('id','createdAt','updatedAt')}
  return body
 count=0
 def compare(method,path,payload=None):
  global count
  values=[call(i,method,path,payload) for i in range(2)]
  assert values[0][0]==values[1][0],(method,path,'status',values)
  assert comparable(values[0][2])==comparable(values[1][2]),(method,path,'body',values[0][2],values[1][2])
  for status,headers,body in values:
   if body is not None:assert headers.get_content_type()=='application/json'
  count+=1
  return values
 compare('GET','/api/providers')
 compare('GET','/api/proxy-pools')
 login=compare('POST','/api/auth/login',{'password':'123456'})
 for _,headers,_ in login:
  cookies=headers.get_all('Set-Cookie') or []
  assert any('auth_token=' in c and 'HttpOnly' in c and 'SameSite=lax' in c.lower().replace('samesite','SameSite') for c in cookies),cookies
 compare('GET','/api/providers')
 compare('POST','/api/provider-nodes',{})
 compare('POST','/api/provider-nodes',{'name':'N','prefix':'p','apiType':'chat'})
 compare('GET','/api/provider-nodes')
 created=compare('POST','/api/providers',{'provider':'openai','name':'N','apiKey':'dummy'})
 ids=[v[2]['connection']['id'] for v in created]
 for method,payload in [('GET',None),('PUT',{'name':'Updated'}),('DELETE',None),('GET',None)]:
  values=[call(i,method,'/api/providers/'+ids[i],payload) for i in range(2)]
  assert values[0][0]==values[1][0] and comparable(values[0][2])==comparable(values[1][2]),values
  count+=1
 compare('POST','/api/proxy-pools',{'name':'Pool','proxyUrl':'http://proxy.test'})
 compare('GET','/api/proxy-pools?includeUsage=true')
 compare('POST','/api/providers/bulk',{'items':[]})
 compare('POST','/api/provider-nodes/validate',{})
 print(f'PASS runtime Next/Go: {count} requests comparadas; guard/login/cookies/CRUD/validacoes')
finally:
 for p in processes:
  p.terminate()
 for p in processes:
  try:p.wait(timeout=10)
  except subprocess.TimeoutExpired:p.kill();p.wait()
 for log in logs:log.close()

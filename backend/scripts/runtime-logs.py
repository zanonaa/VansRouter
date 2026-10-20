"""Next/Go lado a lado, somente no namespace isolado preparado."""
import os, subprocess, json, time, urllib.request, urllib.error, http.cookiejar, sqlite3, tempfile
processes=[]
with tempfile.TemporaryDirectory(prefix='logs-', dir='/scratch/tmp') as root:
 try:
  for name,port,command in [('next',20391,['/node/bin/node','/modules/next/dist/bin/next','dev','--webpack','--hostname','127.0.0.1','--port','20391']),('go',20392,['/scratch/tmp/vansrouter-logs'])]:
   env=dict(os.environ, DATA_DIR=root+'/'+name, PORT=str(port), INITIAL_PASSWORD='123456', TZ='UTC')
   processes.append(subprocess.Popen(command,cwd='/scratch/project',env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL))
  clients=[urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())) for _ in range(2)]
  def call(i,method,path,body=None):
   req=urllib.request.Request(f'http://127.0.0.1:{20391+i}'+path,method=method,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
   try:r=clients[i].open(req,timeout=60)
   except urllib.error.HTTPError as e:r=e
   raw=r.read();return r.status,json.loads(raw) if raw else None
  for i in range(2):
   for _ in range(90):
    try:call(i,'GET','/api/ready');break
    except (OSError,urllib.error.URLError):time.sleep(1)
   else:raise RuntimeError('startup timeout')
  def compare(method,path,body=None):
   a,b=[call(i,method,path,body) for i in range(2)]
   assert a==b,(method,path,a,b)
  compare('GET','/api/usage/request-details')
  compare('POST','/api/auth/login',{'password':'123456'})
  for name in ['next','go']:
   with sqlite3.connect(root+'/'+name+'/db/data.sqlite') as db:
    db.execute('INSERT INTO requestDetails(id,timestamp,provider,data) VALUES(?,?,?,?)',('d','2026-01-01T00:00:00.000Z','openai',json.dumps({'id':'json-d','request':{'dummy':'secret'},'response':None,'extra':True})))
    db.execute('INSERT INTO usageHistory(timestamp,provider,model,connectionId,promptTokens,completionTokens,status,tokens) VALUES(?,?,?,?,?,?,?,?)',('2026-01-01T00:00:00Z',None,None,'connection-id',None,None,None,'{"prompt_tokens":5,"completion_tokens":7}'))
  paths=['/api/usage/logs','/api/usage/request-logs','/api/usage/request-details','/api/usage/request-details?page=0','/api/usage/request-details?pageSize=101','/api/usage/request-details?page=1junk','/api/usage/request-details?endDate=2026-01-01','/api/usage/request-details?provider=missing','/api/usage/request-details/d','/api/usage/request-details/missing']
  for path in paths:compare('GET',path)
  print('PASS runtime logs: 12 requests Next/Go, dados populados/redação/paginação/datas/nulos')
 finally:
  for p in processes:p.terminate()
  for p in processes:
   try:p.wait(timeout=10)
   except subprocess.TimeoutExpired:p.kill();p.wait()

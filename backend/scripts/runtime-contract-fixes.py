import os,subprocess,time,urllib.request,urllib.error,json,http.cookiejar,sqlite3,tempfile
processes=[]
with tempfile.TemporaryDirectory(prefix='contracts-',dir='/scratch/tmp') as root:
 try:
  for name,port,command in [('next',20401,['/node/bin/node','/modules/next/dist/bin/next','dev','--webpack','--hostname','127.0.0.1','--port','20401']),('go',20402,['/scratch/tmp/vansrouter-contracts'])]:
   env=dict(os.environ,DATA_DIR=root+'/'+name,PORT=str(port),INITIAL_PASSWORD='123456',TZ='UTC')
   processes.append(subprocess.Popen(command,cwd='/scratch/project',env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL))
  clients=[urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())) for _ in range(2)]
  def call(i,method,path,body=None):
   req=urllib.request.Request(f'http://127.0.0.1:{20401+i}'+path,method=method,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
   try:r=clients[i].open(req,timeout=60)
   except urllib.error.HTTPError as e:r=e
   raw=r.read();return r.status,json.loads(raw) if raw else None
  for i in range(2):
   for _ in range(90):
    try:call(i,'GET','/api/ready');break
    except (OSError,urllib.error.URLError):time.sleep(1)
   else:raise RuntimeError('startup timeout')
  count=0
  def compare(method,path,body=None):
   global count
   a,b=[call(i,method,path,body) for i in range(2)]
   assert a==b,(method,path,a,b)
   count+=1
  compare('POST','/api/auth/login',{'password':'123456'})
  for ids in [['a','b'],['b','c']]:compare('POST','/api/models/disabled',{'providerAlias':'oa','ids':ids})
  compare('GET','/api/models/disabled')
  for model in ['a','b','c']:compare('DELETE','/api/models/disabled?providerAlias=oa&id='+model)
  compare('GET','/api/models/disabled')
  for name in ['next','go']:
   with sqlite3.connect(root+'/'+name+'/db/data.sqlite') as db:
    for timestamp,key in [('2026-01-02T00:00:00.000Z','sk-long-fictitious-key-1234'),('2026-01-01T00:00:00.000Z','short')]:
     db.execute('INSERT INTO usageHistory(timestamp,provider,model,apiKey,tokens) VALUES(?,?,?,?,?)',(timestamp,'openai','model',key,'{"prompt_tokens":17}'))
  for path in ['/api/usage/history','/api/usage/history?endDate=2026-01-01','/api/usage/history?startDate=2026-01-02','/api/usage/history?startDate=bad','/api/usage/history?provider=none']:compare('GET',path)
  print(f'PASS contract fixes: {count} requests populadas Next/Go')
 finally:
  for p in processes:p.terminate()
  for p in processes:
   try:p.wait(timeout=10)
   except subprocess.TimeoutExpired:p.kill();p.wait()

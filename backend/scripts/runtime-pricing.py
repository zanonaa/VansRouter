import os,subprocess,time,urllib.request,urllib.error,json,http.cookiejar,tempfile
processes=[]
with tempfile.TemporaryDirectory(prefix='pricing-',dir='/scratch/tmp') as root:
 try:
  for name,port,command in [('next',20411,['/node/bin/node','/modules/next/dist/bin/next','dev','--webpack','--hostname','127.0.0.1','--port','20411']),('go',20412,['/scratch/tmp/vansrouter-pricing'])]:
   env=dict(os.environ,DATA_DIR=root+'/'+name,PORT=str(port),INITIAL_PASSWORD='123456',TZ='UTC')
   processes.append(subprocess.Popen(command,cwd='/scratch/project',env=env,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL))
  clients=[urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar())) for _ in range(2)]
  def call(i,method,path,body=None):
   req=urllib.request.Request(f'http://127.0.0.1:{20411+i}'+path,method=method,data=None if body is None else json.dumps(body).encode(),headers={'Content-Type':'application/json'})
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
   assert a==b,(method,path,str(a)[:600],str(b)[:600])
   count+=1
  compare('GET','/api/pricing')
  compare('POST','/api/auth/login',{'password':'123456'})
  compare('GET','/api/pricing')
  for body in [None,{'p':3},{'p':{'m':None}},{'p':{'m':{'bad':1}}},{'p':{'m':{'input':-1}}},{'custom':{'m':{'input':2,'output':3}}},{'custom':{'n':{'cached':4}}}]:compare('PATCH','/api/pricing',body)
  # Invalidações e GET_NEXT têm cache local: PATCH invalida ambos.
  compare('GET','/api/pricing')
  compare('DELETE','/api/pricing?provider=custom&model=m')
  compare('DELETE','/api/pricing?provider=custom')
  compare('DELETE','/api/pricing')
  print(f'PASS runtime pricing: {count} requests Next/Go')
 finally:
  for p in processes:p.terminate()
  for p in processes:
   try:p.wait(timeout=10)
   except subprocess.TimeoutExpired:p.kill();p.wait()

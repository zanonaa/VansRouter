import { readFile, writeFile } from 'node:fs/promises';
// Executa lógica original com dependências isoladas; não inicia Next.js.
const pools=[{id:'a',name:'A',isActive:true},{id:'b',name:'B',isActive:false}];
const connections=[{id:'1',providerSpecificData:{proxyPoolId:'a',proxyPoolIds:['a','a','b']}},{id:'2',providerSpecificData:{proxyPoolIds:['a']}},{id:'3',providerSpecificData:{}}];
async function load(path,method,authorized=true){
 const source=await readFile(new URL(path,import.meta.url),'utf8');
 const code=source.replace(/^import[\s\S]*?from ["'][^"']+["'];/gm,'').replaceAll('export async function','async function');
 return new Function('NextResponse','requireDashboardAuth','createProxyPool','getProxyPools','getProviderConnections','getProxyPoolById','updateProxyPool','deleteProxyPool',code+`\nreturn ${method};`)(
 {json:(body,options={})=>({status:options.status||200,body})},async()=>authorized,async()=>null,
 async filter=>pools.filter(p=>filter.isActive===undefined||p.isActive===filter.isActive),async()=>connections,
 async id=>pools.find(p=>p.id===id)||null,async()=>null,async()=>null);
}
const cases=[];
for(const query of ['', '?includeUsage=true','?isActive=false&includeUsage=true','?isActive=invalid','?includeUsage=false']){
 const handler=await load('../../src/app/api/proxy-pools/route.js','GET');
 cases.push({method:'GET',path:'/api/proxy-pools'+query,...await handler({url:'http://example.test/api/proxy-pools'+query})});
}
for(const method of ['GET','DELETE'])for(const id of ['a','b','missing']){
 const handler=await load('../../src/app/api/proxy-pools/[id]/route.js',method);
 cases.push({method,path:'/api/proxy-pools/'+id,...await handler({}, {params:Promise.resolve({id})})});
}
// b está vinculado acima; adiciona um pool sem vínculos para sucesso no delete.
pools.push({id:'free',name:'Free',isActive:true});
const handler=await load('../../src/app/api/proxy-pools/[id]/route.js','DELETE');
cases.push({method:'DELETE',path:'/api/proxy-pools/free',...await handler({}, {params:Promise.resolve({id:'free'})})});
await writeFile(new URL('../internal/http/handlers/testdata/pool-read-original.json',import.meta.url),JSON.stringify(cases,null,2)+'\n');
console.log(`${cases.length} contratos de leitura/exclusão exportados`);

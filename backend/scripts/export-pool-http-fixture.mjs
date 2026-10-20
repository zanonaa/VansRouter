// Contratos dos handlers originais; NextResponse/auth/DB isolados, sem Next runtime.
import { readFile, writeFile } from 'node:fs/promises';
const response = {json:(body,options={})=>({status:options.status||200,body})};
async function load(path,method) {
 const source=await readFile(new URL(path,import.meta.url),'utf8');
 const executable=source.replace(/^import[\s\S]*?from ["'][^"']+["'];/gm,'').replaceAll('export async function','async function');
 return new Function('NextResponse','requireDashboardAuth','createProxyPool','getProxyPools','getProviderConnections','getProxyPoolById','updateProxyPool','deleteProxyPool',executable+`\nreturn ${method};`)(response,async()=>true,async data=>data,async()=>[],async()=>[],async id=>id==='missing'?null:{id,name:'Existing',proxyUrl:'http://existing.test',isActive:true},async(id,data)=>({id,...data}),async()=>null);
}
const post=await load('../../src/app/api/proxy-pools/route.js','POST');
const put=await load('../../src/app/api/proxy-pools/[id]/route.js','PUT');
const cases=[];
const saved=console.log;console.log=()=>{};
for(const input of [null,{}, {name:12},{name:' P '},{name:' P ',proxyUrl:' http://proxy.test ',type:'invalid'},
 {name:'P',proxyUrl:'http://proxy.test',isActive:'true',strictProxy:1,noProxy:12},
 {name:'P',proxyUrl:'http://proxy.test',isActive:false,strictProxy:true,type:'cloudflare'},
]) cases.push({method:'POST',id:'',input,...await post({json:async()=>input})});
for(const input of [null,{}, {name:'  '},{proxyUrl:''},{name:' N ',noProxy:' local ',isActive:'true',strictProxy:true,type:'deno'},{name:12},{proxyUrl:42},{type:'other'}])
 cases.push({method:'PUT',id:'pool',input,...await put({json:async()=>input},{params:Promise.resolve({id:'pool'})})});
cases.push({method:'PUT',id:'missing',input:{name:'N'},...await put({json:async()=>({name:'N'})},{params:Promise.resolve({id:'missing'})})});
console.log=saved;
await writeFile(new URL('../internal/http/handlers/testdata/pool-http-original.json',import.meta.url),JSON.stringify(cases,null,2)+'\n');
console.log(`${cases.length} contratos de proxyPools exportados`);

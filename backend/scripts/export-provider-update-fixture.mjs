import {readFile,writeFile} from 'node:fs/promises';
const source=await readFile(new URL('../../src/app/api/providers/[id]/route.js',import.meta.url),'utf8');
const executable=source.replace(/^import[\s\S]*?from ["'][^"']+["'];/gm,'').replaceAll('export async function','async function');
const existing={id:'conn',provider:'openai',authType:'apikey',name:'Existing',apiKey:'secret',accessToken:'access',refreshToken:'refresh',idToken:'id-secret',providerSpecificData:{custom:true,proxyPoolId:'old'}};
const repo=new Function('NextResponse','getProviderConnectionById','getProxyPoolById','updateProviderConnection','deleteProviderConnection',executable+'\nreturn {GET,PUT,DELETE};')(
 {json:(body,options={})=>({status:options.status||200,body})},async id=>id==='missing'?null:structuredClone(existing),async id=>id==='pool'?{id}:null,async(id,patch)=>({...structuredClone(existing),...patch}),async id=>id!=='missing');
const cases=[];const saved=console.log;console.log=()=>{};
for(const input of [{},{name:'New',apiKey:'replacement'},{connectionProxyEnabled:true},{connectionProxyUrl:' http://proxy ',connectionNoProxy:' local '},{proxyPoolId:'missing'},{proxyPoolId:null},{proxyPoolIds:['pool','pool'],proxyRotationStrategy:' SMART '},{proxyPoolIds:[],proxyRotationStrategy:'invalid'},{providerSpecificData:[1,2]},{providerSpecificData:'xy'}]){
 cases.push({method:'PUT',id:'conn',input,...await repo.PUT({json:async()=>input},{params:Promise.resolve({id:'conn'})})});
}
for(const method of ['GET','DELETE'])for(const id of ['conn','missing'])cases.push({method,id,input:null,...await repo[method]({}, {params:Promise.resolve({id})})});
console.log=saved;
await writeFile(new URL('../internal/http/handlers/testdata/provider-update-original.json',import.meta.url),JSON.stringify(cases,null,2)+'\n');
console.log(`${cases.length} contratos provider por ID exportados`);

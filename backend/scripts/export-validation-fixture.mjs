import {readFile,writeFile} from 'node:fs/promises';
const source=await readFile(new URL('../../src/app/api/provider-nodes/validate/route.js',import.meta.url),'utf8');
const executable=source.replace(/^import .*;\r?$/gm,'').replaceAll('export async function','async function');
const cases=[];const saved=console.error;console.error=()=>{};
for(const test of [
 {input:{},statuses:[]},
 {input:{baseUrl:'not a url',apiKey:'x'},statuses:[]},
 {input:{baseUrl:'http://example.test',apiKey:'x'},statuses:[200]},
 {input:{baseUrl:'http://example.test',apiKey:'x'},statuses:[401]},
 {input:{baseUrl:'http://example.test',apiKey:'x'},statuses:[404]},
 {input:{baseUrl:'http://example.test',apiKey:'x',modelId:'m'},statuses:[404,200]},
 {input:{baseUrl:'http://example.test',apiKey:'x',type:'anthropic-compatible',modelId:'m'},statuses:[404,400]},
 {input:{baseUrl:'http://example.test',apiKey:'x',type:'custom-embedding'},statuses:[]},
 {input:{baseUrl:'http://example.test',apiKey:'x',type:'custom-embedding',modelId:'m'},statuses:[200],data:{data:[{embedding:[1,2]}]}},
 {input:{baseUrl:'http://example.test',apiKey:'x',type:'custom-embedding',modelId:'m'},statuses:[503],text:'error'},
]){
 let index=0;
 const handler=new Function('NextResponse','assertPublicUrl','isLocalRequest','ANTHROPIC_API_VERSION','fetch','setTimeout',executable+'\nreturn POST;')(
 {json:(body,options={})=>({status:options.status||200,body})},async()=>{},()=>true,'2023-06-01',async()=>{const status=test.statuses[index++];return {status,ok:status>=200&&status<300,json:async()=>test.data||{},text:async()=>test.text||''};},()=>0);
 cases.push({...test,...await handler({json:async()=>test.input})});
}
console.error=saved;
await writeFile(new URL('../internal/http/handlers/testdata/validation-original.json',import.meta.url),JSON.stringify(cases,null,2)+'\n');
console.log(`${cases.length} contratos de validação exportados`);

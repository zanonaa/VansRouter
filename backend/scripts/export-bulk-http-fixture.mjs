import {readFile,writeFile} from 'node:fs/promises';
import registry from '../../open-sse/providers/registry/index.js';
const category=c=>Object.fromEntries(registry.filter(r=>r.category===c).map(r=>[r.id,r]));
const normalize=value=>typeof value==='string'?value.trim().toLowerCase():value;
const source=await readFile(new URL('../../src/app/api/providers/bulk/route.js',import.meta.url),'utf8');
const code=source.replace(/^import[\s\S]*?from ["'][^"']+["'];/gm,'').replaceAll('export async function','async function');
const handler=new Function('NextResponse','createProviderConnectionsBulk','getProviderNodeById','FREE_TIER_PROVIDERS','WEB_COOKIE_PROVIDERS','isOpenAICompatibleProvider','isAnthropicCompatibleProvider','isCustomEmbeddingProvider','APIKEY_PROVIDERS','normalizeProviderId',code+'\nreturn POST;')(
 {json:(body,options={})=>({status:options.status||200,body})},async items=>items,async id=>id.endsWith('-known')?{prefix:'n',apiType:'chat',baseUrl:'https://node.test/v1',name:'Node'}:null,
 category('freeTier'),category('webCookie'),p=>typeof p==='string'&&p.startsWith('openai-compatible-'),p=>typeof p==='string'&&p.startsWith('anthropic-compatible-'),p=>typeof p==='string'&&p.startsWith('custom-embedding-'),category('apikey'),normalize);
const inputs=[{}, {items:[]}, {items:[{provider:'openai',name:'N'}]}, {items:[{provider:'openai',name:'N',apiKey:'x'},{provider:'openai',name:' N ',apiKey:'y'}]},
 {items:[{provider:'openai',name:' N ',apiKey:' x ',priority:2.5}]},
 {items:[{provider:'custom-embedding-missing',name:'N',apiKey:'x'}]},
 {items:[{provider:'openai-compatible-known',name:'N',apiKey:'x',providerSpecificData:{extra:true}}]},
 {items:[{provider:'anthropic-compatible-known',name:'N',apiKey:'x',providerSpecificData:'abc'}]},
 {items:[{provider:'openai',name:'N',apiKey:'x',priority:0}]},
];
const cases=[];for(const input of inputs)cases.push({input,...await handler({json:async()=>input})});
await writeFile(new URL('../internal/http/handlers/testdata/bulk-http-original.json',import.meta.url),JSON.stringify(cases,null,2)+'\n');

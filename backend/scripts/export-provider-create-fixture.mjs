import {readFile,writeFile} from 'node:fs/promises';
import registry from '../../open-sse/providers/registry/index.js';
const entries={};for(const category of ['free','freeTier','oauth','apikey','webCookie'])for(const entry of registry.filter(r=>r.category===category))entries[entry.id]={...entry.display,...entry,alias:entry.uiAlias||entry.alias,display:undefined};
const category=c=>Object.fromEntries(registry.filter(r=>r.category===c).map(r=>[r.id,entries[r.id]]));
const compatible=p=>typeof p==='string'&&p.startsWith('openai-compatible-');
const anthropic=p=>typeof p==='string'&&p.startsWith('anthropic-compatible-');
const embedding=p=>typeof p==='string'&&p.startsWith('custom-embedding-');
const normalize=p=>{if(typeof p!=='string')return p;const trimmed=p.trim();if(entries[trimmed])return trimmed;const slug=trimmed.toLowerCase().replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'');if(entries[slug])return slug;return Object.values(entries).find(e=>e.name?.toLowerCase()===trimmed.toLowerCase())?.id||trimmed;};
const specific=(provider,body,incoming)=>{const next=incoming&&typeof incoming==='object'?{...incoming}:{};if(provider==='ollama-local'){const base=(next.baseUrl||body.baseUrl||body.baseURL||body.ollamaHostUrl||'').trim();if(base)next.baseUrl=base;}return Object.keys(next).length?next:null;};
const source=await readFile(new URL('../../src/app/api/providers/route.js',import.meta.url),'utf8');
const executable=source.replace(/^import[\s\S]*?from ["'][^"']+["'];/gm,'').replace(/^export const .*;\r?$/gm,'').replaceAll('export async function','async function');
const post=new Function('NextResponse','getProviderConnections','createProviderConnection','getProviderNodeById','getProviderNodes','getProxyPoolById','APIKEY_PROVIDERS','AI_PROVIDERS','FREE_TIER_PROVIDERS','WEB_COOKIE_PROVIDERS','isOpenAICompatibleProvider','isAnthropicCompatibleProvider','isCustomEmbeddingProvider','normalizeProviderId','normalizeProviderSpecificData',executable+'\nreturn POST;')(
 {json:(body,options={})=>({status:options.status||200,body})},async()=>[],async data=>data,async id=>id.endsWith('-known')?{id,name:'Node',prefix:'n',baseUrl:'https://node.test/v1',apiType:'chat'}:null,async()=>[],async id=>id==='pool'?{id}:null,
 category('apikey'),entries,category('freeTier'),category('webCookie'),compatible,anthropic,embedding,normalize,specific);
const inputs=[{}, {provider:'openai'}, {provider:' OpenAI ',apiKey:'dummy',name:'N'}, {provider:'openai',apiKey:'dummy',connectionProxyEnabled:true}, {provider:'openai',apiKey:'dummy',proxyPoolId:'missing'}, {provider:'openai',apiKey:'dummy',proxyPoolId:'pool',providerSpecificData:[1,2]}, {provider:'ollama-local',baseURL:' http://local.test '}, {provider:'custom-embedding-missing',apiKey:'dummy',name:'N'}, {provider:'openai-compatible-known',apiKey:'dummy',name:'N'}, {provider:'anthropic-compatible-known',apiKey:'dummy',name:'N'}, {provider:'openai',apiKey:'dummy',priority:0,globalPriority:0,providerSpecificData:'ignored'}];
const saved=console.log;console.log=()=>{};const cases=[];for(const input of inputs)cases.push({input,...await post({json:async()=>input})});console.log=saved;
await writeFile(new URL('../internal/http/handlers/testdata/provider-create-original.json',import.meta.url),JSON.stringify(cases,null,2)+'\n');console.log(`${cases.length} contratos POST providers exportados`);

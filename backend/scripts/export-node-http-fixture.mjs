// Executa o handler original com NextResponse.json e persistência isolados.
// Não inicia Next.js: compara lógica HTTP do handler, não middleware/runtime.
import { readFile, writeFile } from 'node:fs/promises';
const source=await readFile(new URL('../../src/app/api/provider-nodes/route.js',import.meta.url),'utf8');
const executable=source.replace(/^import .*;\r?$/gm,'').replace(/^export const .*;\r?$/gm,'').replaceAll('export async function','async function');
const {POST}=new Function('NextResponse','createProviderNode','getProviderNodes','OPENAI_COMPATIBLE_PREFIX','ANTHROPIC_COMPATIBLE_PREFIX','CUSTOM_EMBEDDING_PREFIX','randomUUID',executable+'\nreturn {POST};')(
 {json:(body,options={})=>({status:options.status||200,body})},async data=>data,async()=>[],
 'openai-compatible-','anthropic-compatible-','custom-embedding-',()=> 'fixture-uuid');
const inputs=[{}, {name:'N'}, {name:'N',prefix:'p'},
 {name:'N',prefix:'p',type:'invalid'}, {name:12,prefix:'p'},
 {name:' N ',prefix:' p ',apiType:'chat'},
 {name:'N',prefix:'p',apiType:'responses',baseUrl:' https://example.test/v1/ '},
 {name:'N',prefix:'p',type:'anthropic-compatible',baseUrl:' https://example.test/messages/ '},
 {name:'N',prefix:'p',type:'custom-embedding',baseUrl:'https://example.test/v1/embeddings/'},
 {name:'N',prefix:'p',type:'custom-embedding',baseUrl:'   '},
 {name:'N',prefix:'p',apiType:'chat',baseUrl:42},
];
// O handler registra erros esperados de tipo; não imprimir stack no gerador.
const saved=console.log;console.log=()=>{};
const fixtures=[];
for(const input of inputs){const response=await POST({json:async()=>input});fixtures.push({input,...response});}
console.log=saved;
await writeFile(new URL('../internal/http/handlers/testdata/node-http-original.json',import.meta.url),JSON.stringify(fixtures,null,2)+'\n');
console.log(`${fixtures.length} contratos do handler original exportados`);

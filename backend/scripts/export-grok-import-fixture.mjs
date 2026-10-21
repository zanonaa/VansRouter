// Executa o handler original; apenas persistência/NextResponse/claims isolados.
import {readFile,writeFile} from 'node:fs/promises';
const source=await readFile(new URL('../../src/app/api/oauth/grok-cli/bulk-import/route.js',import.meta.url),'utf8');
const code=source.replace(/^import .*;\r?$/gm,'').replaceAll('export async function','async function');
const cases=[];
for(const input of [null,[],{},[null,{},'bad'],{access_token:'dummy',email:'op@example.test',expires_at:'2027-01-01T00:00:00Z'},{accounts:[{accessToken:'first',name:'N',providerSpecificData:{authMethod:'manual',custom:true}},{access_token:'second',refresh_token:'refresh',id_token:'id-dummy'}]}]){
 const saved=[];
 const handler=new Function('NextResponse','createProviderConnection','decodeXaiIdTokenEmail','extractEmailFromAccessToken',code+'\nreturn POST;')({json:(body,options={})=>({status:options.status||200,body})},async data=>{saved.push(data);return {id:'saved-'+saved.length,...data};},()=>null,()=>null);
 const result=await handler({json:async()=>input});cases.push({input,...result,saved:JSON.parse(JSON.stringify(saved))});
}
await writeFile(new URL('../internal/http/handlers/testdata/grok-import-original.json',import.meta.url),JSON.stringify(cases,null,2)+'\n');

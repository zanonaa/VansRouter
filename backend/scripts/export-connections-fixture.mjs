import { DatabaseSync } from 'node:sqlite';
import { readFile, writeFile } from 'node:fs/promises';
const db = new DatabaseSync(':memory:');
db.exec(`CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT NOT NULL,authType TEXT NOT NULL,name TEXT,email TEXT,priority INTEGER,isActive INTEGER,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL)`);
const adapter = {
 run(sql,args=[]) { return db.prepare(sql).run(...args.map(v=>v===undefined?null:v)); },
 get(sql,args=[]) { return db.prepare(sql).get(...args); },
 all(sql,args=[]) { return db.prepare(sql).all(...args); },
 transaction(fn) { db.exec('BEGIN');try{const v=fn();db.exec('COMMIT');return v;}catch(e){db.exec('ROLLBACK');throw e;} },
};
const source=await readFile(new URL('../../src/lib/db/repos/connectionsRepo.js',import.meta.url),'utf8');
const executable=source.replace(/^import .*;\r?$/gm,'').replaceAll('export async function','async function');
let sequence=0;
const methods=['createProviderConnection','getProviderConnectionById','getProviderConnections','updateProviderConnection','createProviderConnectionsBulk','deleteProviderConnection','cleanupProviderConnections'];
const repo=new Function('getAdapter','randomUUID','parseJson','stringifyJson',executable+'\nreturn {'+methods.join(',')+'};')(async()=>adapter,()=>`generated-${++sequence}`,(v,f)=>{try{return JSON.parse(v);}catch{return f;}},v=>JSON.stringify(v??null));
const ids=new Map();
function stable(value){
 if(Array.isArray(value))return value.map(stable);
 if(value && typeof value==='object')return Object.fromEntries(Object.entries(value).filter(([k,v])=>!['createdAt','updatedAt'].includes(k)&&v!==undefined).map(([k,v])=>[k,k==='id'?ids.get(v)||v:stable(v)]));
 return value;
}
const references={};const steps=[];
async function step(method,args,save=''){
 const actual=args.map(v=>typeof v==='string'&&v.startsWith('$')?references[v.slice(1)]:v);
 const result=await repo[method](...actual);
 if(save){references[save]=result.id;ids.set(result.id,save);}
 steps.push({method,args,save,expected:stable(result)});
}
await step('createProviderConnection',[{provider:'openai',authType:'apikey',name:'A',apiKey:'first',lastError:'old',testStatus:'error'}],'a');
await step('createProviderConnection',[{provider:'openai',authType:'apikey',name:'B',apiKey:'second'}],'b');
await step('createProviderConnection',[{provider:'openai',authType:'apikey',name:'A',apiKey:'replacement',testStatus:'active'}]);
await step('getProviderConnectionById',['$a']);
await step('updateProviderConnection',['$a',{providerSpecificData:{custom:true},modelLock_test:'locked',unknown:'keep'}]);
await step('updateProviderConnection',['$a',{testStatus:'active'}]);
await step('getProviderConnections',[{provider:'openai',isActive:true}]);
await step('createProviderConnection',[{provider:'codex',authType:'oauth',email:'same@example.test'}],'codex1');
await step('createProviderConnection',[{provider:'codex',authType:'oauth',email:'same@example.test'}],'codex2');
await step('createProviderConnection',[{provider:'github',authType:'oauth',email:'one@example.test',providerSpecificData:{githubLogin:'Operator'}}],'github');
await step('createProviderConnectionsBulk',[[{provider:'openai',name:'A',apiKey:'bulk-replaced'},{provider:'openai',name:'invalid'}]]);
await step('getProviderConnectionById',['$a']);
await step('deleteProviderConnection',['$b']);
await step('getProviderConnections',[{provider:'openai'}]);
await step('deleteProviderConnection',['missing']);
await step('cleanupProviderConnections',[]);
await step('getProviderConnectionById',['$a']);
await writeFile(new URL('../internal/database/testdata/connections-original.json',import.meta.url),JSON.stringify(steps,null,2)+'\n');
db.close();console.log(`${steps.length} cenários connections originais exportados`);

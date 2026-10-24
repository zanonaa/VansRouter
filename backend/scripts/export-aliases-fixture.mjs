import {DatabaseSync} from 'node:sqlite';
import {readFile,writeFile} from 'node:fs/promises';
const db=new DatabaseSync(':memory:');db.exec('CREATE TABLE kv(scope TEXT NOT NULL,key TEXT NOT NULL,value TEXT NOT NULL,PRIMARY KEY(scope,key))');
const adapter={run:(q,a=[])=>db.prepare(q).run(...a.map(v=>v===undefined?null:v)),get:(q,a=[])=>db.prepare(q).get(...a),all:(q,a=[])=>db.prepare(q).all(...a),transaction(fn){db.exec('BEGIN');try{const r=fn();db.exec('COMMIT');return r;}catch(e){db.exec('ROLLBACK');throw e;}}};
const makeKv=scope=>({async getAll(){const rows=adapter.all('SELECT key,value FROM kv WHERE scope = ?',[scope]);const out={};for(const r of rows)out[r.key]=JSON.parse(r.value);return out;},async set(key,value){adapter.run(`INSERT INTO kv(scope,key,value) VALUES(?,?,?) ON CONFLICT(scope,key) DO UPDATE SET value=excluded.value`,[scope,key,JSON.stringify(value??null)]);},async remove(key){adapter.run('DELETE FROM kv WHERE scope = ? AND key = ?',[scope,key]);}});

const source=await readFile(new URL('../../src/lib/db/repos/aliasRepo.js',import.meta.url),'utf8');const code=source.replace(/^import .*;\r?$/gm,'').replaceAll('export async function','async function');
const repo=new Function('getAdapter','parseJson','stringifyJson','makeKv',code+'\nreturn {getModelAliases,setModelAlias,deleteModelAlias,getCustomModels,addCustomModel,addCustomModelsBulk,deleteCustomModel};')(async()=>adapter,(v,f)=>{try{return JSON.parse(v);}catch{return f;}},v=>JSON.stringify(v??null),makeKv);
const steps=[];const snapshot=()=>({aliases:structuredClone(db.prepare('SELECT scope,key,value FROM kv ORDER BY scope,key').all())});
async function step(method,args){steps.push({method,args,expected:await repo[method](...args),after:snapshot()});}
await step('getModelAliases',[]);await step('setModelAlias',['fast','openai/gpt']);await step('setModelAlias',['fast','openai/gpt-2']);await step('getModelAliases',[]);await step('setModelAlias',['bad',12345]);
await step('addCustomModel',[{providerAlias:'oa',id:'m',caps:{vision:true,bad:'x'},transport:'gemini-live'}]);
await step('addCustomModel',[{providerAlias:'oa',id:'m',name:'Updated',caps:{reasoning:true}}]);
await step('getCustomModels',[]);
await step('addCustomModelsBulk',[[{providerAlias:'oa',id:'b',name:'B'},{providerAlias:'oa',id:'b'},{id:'skip'},{providerAlias:'oa',id:'c'}]]);
await step('addCustomModel',[{providerAlias:'oa',id:'stt',type:'stt',transport:'gemini-live'}]);
await step('deleteModelAlias',['fast']);await step('getModelAliases',[]);
await step('deleteCustomModel',[{providerAlias:'oa',id:'m'}]);await step('getCustomModels',[]);
await step('deleteCustomModel',[{providerAlias:'oa',id:'missing'}]);
await writeFile(new URL('../internal/database/testdata/aliases-original.json',import.meta.url),JSON.stringify(steps,null,2)+'\n');db.close();

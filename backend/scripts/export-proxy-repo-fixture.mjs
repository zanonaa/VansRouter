// Executa os repositórios originais com adapter SQLite em memória.
import { DatabaseSync } from 'node:sqlite';
import { readFile, writeFile } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
const db = new DatabaseSync(':memory:');
db.exec(`CREATE TABLE proxyPools(id TEXT PRIMARY KEY,isActive INTEGER,testStatus TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);
CREATE TABLE proxyPoolFitness(poolId TEXT NOT NULL,scope TEXT NOT NULL,until INTEGER NOT NULL,reason TEXT,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL,PRIMARY KEY(poolId,scope));`);
const adapter = {
  run(sql,args=[]) { return db.prepare(sql).run(...args); },
  get(sql,args=[]) { return db.prepare(sql).get(...args); },
  all(sql,args=[]) { return db.prepare(sql).all(...args); },
  transaction(fn) { db.exec('BEGIN'); try { const value=fn(); db.exec('COMMIT'); return value; } catch(error) { db.exec('ROLLBACK'); throw error; } },
};
async function load(file,names) {
  const source=await readFile(new URL('../../src/lib/db/repos/'+file,import.meta.url),'utf8');
  const executable=source.replace(/^import .*;\r?$/gm,'').replaceAll('export async function','async function');
  return new Function('getAdapter','randomUUID','parseJson','stringifyJson',executable+'\nreturn {'+names.join(',')+'};')(async()=>adapter,randomUUID,(v,fallback)=>{try{return JSON.parse(v);}catch{return fallback;}},v=>JSON.stringify(v??null));
}
const pools=await load('proxyPoolsRepo.js',['createProxyPool','getProxyPoolById','getProxyPools','updateProxyPool','deleteProxyPool']);
const fitness=await load('proxyPoolFitnessRepo.js',['listProxyPoolFitness','upsertProxyPoolFitness','deleteProxyPoolFitness','clearProxyPoolFitness','deleteProxyPoolFitnessByPool']);
const repo={...pools,...fitness};
function stable(value) {
  if(Array.isArray(value)) return value.map(stable);
  if(value && typeof value==='object') return Object.fromEntries(Object.entries(value).filter(([k,v])=>!['createdAt','updatedAt'].includes(k)&&v!==undefined).map(([k,v])=>[k,stable(v)]));
  return value;
}
const steps=[];
async function step(method,args,changes=false) {
  let value=await repo[method](...args);
  if(changes) value=Number(value.changes);
  steps.push({method,args,expected:stable(value)});
}
await step('createProxyPool',[{id:'a',name:'A',proxyUrl:'http://proxy.example.test',strictProxy:true}]);
await step('createProxyPool',[{id:'b',name:'B',isActive:false,type:'deno'}]);
await step('getProxyPoolById',['a']);
await step('updateProxyPool',['a',{testStatus:'active',unknown:{keep:true},lastError:null}]);
await step('getProxyPools',[{isActive:true,testStatus:'active'}]);
await step('getProxyPools',[{isActive:false}]);
await step('updateProxyPool',['missing',{name:'X'}]);
await step('upsertProxyPoolFitness',['a','openai::model',1790000000000,'limit']);
await step('upsertProxyPoolFitness',['a','openai::model',1790000000001,'']);
await step('upsertProxyPoolFitness',['b','other::*',0,'']);
await step('upsertProxyPoolFitness',['','invalid',1,'']);
await step('listProxyPoolFitness',[]);
await step('listProxyPoolFitness',['a']);
await step('deleteProxyPoolFitness',['a','missing'],true);
await step('clearProxyPoolFitness',['openai'],true);
await step('listProxyPoolFitness',[]);
await step('upsertProxyPoolFitness',['a','scope',1,'']);
await step('deleteProxyPool',['a']);
await step('listProxyPoolFitness',[]);
await step('deleteProxyPool',['a']);
await step('deleteProxyPoolFitnessByPool',['b'],true);
await step('clearProxyPoolFitness',[],true);
await writeFile(new URL('../internal/database/testdata/proxies-original.json',import.meta.url),JSON.stringify(steps,null,2)+'\n');
db.close();
console.log(`${steps.length} resultados gerados pelos repositórios originais de proxies`);

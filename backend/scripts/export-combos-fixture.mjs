import {DatabaseSync} from 'node:sqlite';
import {readFile,writeFile} from 'node:fs/promises';
const db=new DatabaseSync(':memory:');db.exec('CREATE TABLE combos(id TEXT PRIMARY KEY,name TEXT UNIQUE NOT NULL,kind TEXT,models TEXT NOT NULL,context_length INTEGER,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL)');
const adapter={run:(q,a=[])=>db.prepare(q).run(...a),get:(q,a=[])=>db.prepare(q).get(...a),all:(q,a=[])=>db.prepare(q).all(...a),transaction(fn){db.exec('BEGIN');try{const r=fn();db.exec('COMMIT');return r;}catch(e){db.exec('ROLLBACK');throw e;}}};
const source=await readFile(new URL('../../src/lib/db/repos/combosRepo.js',import.meta.url),'utf8');const code=source.replace(/^import .*;\r?$/gm,'').replaceAll('export async function','async function');
const repo=new Function('getAdapter','randomUUID','parseJson','stringifyJson',code+'\nreturn {getCombos,getComboById,getComboByName,createCombo,updateCombo,deleteCombo};')(async()=>adapter,()=> 'fixture-id',(v,f)=>{try{return JSON.parse(v);}catch{return f;}},v=>JSON.stringify(v??null));
function stable(v){if(Array.isArray(v))return v.map(stable);if(v&&typeof v==='object')return Object.fromEntries(Object.entries(v).filter(([k])=>!['createdAt','updatedAt'].includes(k)).map(([k,item])=>[k,stable(item)]));return v;}
const steps=[];async function step(method,args){steps.push({method,args,expected:stable(await repo[method](...args))});}
await step('getCombos',[]);await step('createCombo',[{name:'combo',models:['openai/model'],context_length:4096}]);await step('getComboByName',['combo']);await step('getComboById',['missing']);await step('updateCombo',['fixture-id',{name:'renamed',context_length:null,models:[],extra:'returned-only'}]);await step('getComboById',['fixture-id']);await step('deleteCombo',['fixture-id']);await step('deleteCombo',['fixture-id']);
const handlerSource=await readFile(new URL('../../src/app/api/combos/[id]/route.js',import.meta.url),'utf8');
const start=handlerSource.indexOf('export function validateContextLength');const end=handlerSource.indexOf('// GET /api/combos',start);
const validate=new Function('MAX_CONTEXT_LENGTH',handlerSource.slice(start,end).replace('export function','function')+'\nreturn validateContextLength;')(2000000);
const values=[null,'',true,false,0,-1,1.5,4096,'4096','0x100','0b100','0o10','  ',[],[12],{},2000001];
await writeFile(new URL('../internal/database/testdata/combos-original.json',import.meta.url),JSON.stringify(steps,null,2)+'\n');
await writeFile(new URL('../internal/http/handlers/testdata/context-length-original.json',import.meta.url),JSON.stringify(values.map(input=>({input,expected:validate(input)})),null,2)+'\n');db.close();

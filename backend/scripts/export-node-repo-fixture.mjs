// Executa nodesRepo.js original com apenas getAdapter substituído por SQLite
// em memória. Sem modificar código de produção ou ler DATA_DIR do operador.
import { DatabaseSync } from 'node:sqlite';
import { readFile, writeFile } from 'node:fs/promises';
import { randomUUID } from 'node:crypto';
const db = new DatabaseSync(':memory:');
db.exec('CREATE TABLE providerNodes(id TEXT PRIMARY KEY,type TEXT,name TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL)');
const adapter = {
  run(sql, args = []) { return db.prepare(sql).run(...args); },
  get(sql, args = []) { return db.prepare(sql).get(...args); },
  all(sql, args = []) { return db.prepare(sql).all(...args); },
  transaction(fn) { db.exec('BEGIN'); try { const result = fn(); db.exec('COMMIT'); return result; } catch (error) { db.exec('ROLLBACK'); throw error; } },
};
const source = await readFile(new URL('../../src/lib/db/repos/nodesRepo.js', import.meta.url), 'utf8');
const executable = source.replace(/^import .*;\r?$/gm, '').replaceAll('export async function', 'async function');
const factory = new Function('getAdapter', 'randomUUID', 'parseJson', 'stringifyJson', executable + '\nreturn {createProviderNode,getProviderNodeById,getProviderNodes,updateProviderNode,deleteProviderNode};');
const repo = factory(async () => adapter, randomUUID, (v, fallback) => { try { return JSON.parse(v); } catch { return fallback; } }, v => JSON.stringify(v ?? null));
function stable(value) {
  if (Array.isArray(value)) return value.map(stable);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).filter(([k,v]) => !['createdAt','updatedAt'].includes(k) && v !== undefined).map(([k,v]) => [k,stable(v)]));
  return value;
}
const steps = [];
async function step(method, args) { steps.push({ method, args, expected: stable(await repo[method](...args)) }); }
await step('createProviderNode', [{id:'n',type:'openai-compatible',name:'Node',prefix:'p',baseUrl:'https://example.test/v1',apiType:'chat',ignored:'not stored'}]);
await step('getProviderNodeById', ['n']);
await step('updateProviderNode', ['n',{name:'Updated',extra:{preserved:true},prefix:null}]);
await step('getProviderNodes', [{type:'other'}]);
await step('getProviderNodes', [{type:'openai-compatible'}]);
await step('getProviderNodeById', ['missing']);
await step('deleteProviderNode', ['n']);
await step('deleteProviderNode', ['n']);
await writeFile(new URL('../internal/database/testdata/nodes-original.json', import.meta.url), JSON.stringify(steps,null,2)+'\n');
db.close();
console.log(`${steps.length} resultados gerados pelo nodesRepo.js original`);

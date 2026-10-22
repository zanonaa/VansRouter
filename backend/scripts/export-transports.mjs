import fs from 'node:fs';
import {PROVIDERS} from '../../open-sse/providers/index.js';
const entries={};
for(const [id,transport] of Object.entries(PROVIDERS)){
 if(typeof transport.baseUrl!=='string')continue;
 entries[id]={baseUrl:transport.baseUrl,format:transport.format,headers:transport.headers || {},auth:transport.auth || null,noAuth:transport.noAuth===true};
}
fs.writeFileSync(new URL('../internal/providers/transports.json',import.meta.url),JSON.stringify(entries,null,2)+'\n');

import fs from 'node:fs';
import {PROVIDERS,PROVIDER_MEDIA} from '../../open-sse/providers/index.js';
import {deriveValidateUrl} from '../../open-sse/providers/schema.js';
const data={};
for(const [id,transport] of Object.entries(PROVIDERS)){
 const catalog=PROVIDER_MEDIA[id]?.modelsFetcher;
 const url=catalog?.url || deriveValidateUrl(transport);
 if(url)data[id]={url,public:catalog?.public===true,authHeader:transport.auth?.header || 'Authorization',authPrefix:!transport.auth || transport.auth.scheme==='bearer'?'Bearer ':''};
}
fs.writeFileSync(new URL('../internal/providers/discovery.json',import.meta.url),JSON.stringify(data,null,2)+'\n');

import { PROVIDERS } from '../../open-sse/config/providers.js';
import {writeFile} from 'node:fs/promises';
const headers=Object.fromEntries(Object.entries(PROVIDERS).map(([id,p])=>[id,p.headers||{}]));
await writeFile(new URL('../internal/providers/builtin-headers.json',import.meta.url),JSON.stringify(headers,null,2)+'\n');

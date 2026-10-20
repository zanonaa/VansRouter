import { PROVIDER_PRICING } from '../../open-sse/providers/pricing.js';
import { writeFile } from 'node:fs/promises';
await writeFile(new URL('../internal/providers/pricing.json',import.meta.url),JSON.stringify(PROVIDER_PRICING,null,2)+'\n');

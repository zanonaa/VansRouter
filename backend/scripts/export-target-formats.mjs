import fs from 'node:fs';
import { PROVIDERS } from '../../open-sse/providers/index.js';
const entries = {};
for (const [id, transport] of Object.entries(PROVIDERS)) {
  if (typeof transport.format === 'string') entries[id] = transport.format;
}
fs.writeFileSync(new URL('../internal/providers/target-formats.json', import.meta.url), JSON.stringify(entries, null, 2) + '\n');

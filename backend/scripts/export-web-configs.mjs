import fs from 'node:fs';
import { PROVIDER_MEDIA } from '../../open-sse/providers/index.js';
import REGISTRY from '../../open-sse/providers/registry/index.js';
const out = {};
const aliases = Object.fromEntries(REGISTRY.map(provider => [provider.id, provider.alias || provider.id]));
for (const [id, provider] of Object.entries(PROVIDER_MEDIA)) {
  if (!provider.searchConfig && !provider.fetchConfig && !provider.videoConfig) continue;
  out[id] = {
    alias: aliases[id] || id,
    noAuth: provider.noAuth === true,
    search: provider.searchConfig || null,
    fetch: provider.fetchConfig || null,
    video: provider.videoConfig || null,
  };
}
fs.writeFileSync(new URL('../internal/providers/web-configs.json', import.meta.url), JSON.stringify(out, null, 2) + '\n');

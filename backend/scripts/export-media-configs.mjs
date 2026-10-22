import fs from 'node:fs';
import { PROVIDER_MEDIA } from '../../open-sse/providers/index.js';
const result = {};
for (const [id, provider] of Object.entries(PROVIDER_MEDIA)) {
  for (const [source, target] of [['ttsConfig', 'tts'], ['sttConfig', 'stt']]) {
    const config = provider[source];
    if (!config?.baseUrl || !config.format) continue;
    result[id] ??= {};
    result[id][target] = { baseUrl: config.baseUrl, authType: config.authType || '', authHeader: config.authHeader || '', format: config.format };
  }
}
fs.writeFileSync(new URL('../internal/providers/media-configs.json', import.meta.url), JSON.stringify(result, null, 2) + '\n');

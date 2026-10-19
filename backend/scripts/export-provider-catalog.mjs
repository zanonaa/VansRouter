// Executar da raiz do repositório: node backend/scripts/export-provider-catalog.mjs
// Só exporta metadados públicos necessários ao CRUD; nunca exporta transport/OAuth.
import registry from '../../open-sse/providers/registry/index.js';
import { writeFile } from 'node:fs/promises';
const categories = ['free', 'freeTier', 'oauth', 'apikey', 'webCookie'];
const providers = [];
for (const category of categories) {
  for (const entry of registry.filter(r => r.category === category)) {
    const alias = entry.uiAlias || entry.alias;
    const media = entry.media || {};
    providers.push({
      id: entry.id, category, alias: alias || null, aliases: entry.aliases || [],
      name: entry.display?.name || '',
      // buildProviderEntry espalha display; não possui propriedade display.
      // GET /api/providers consulta def.display?.name e portanto devolve o ID.
      displayName: entry.id,
      noAuth: !!entry.noAuth,
      serviceKinds: entry.serviceKinds ?? media.serviceKinds ?? [],
      authModes: entry.authModes || [],
    });
  }
}
if (new Set(providers.map(p => p.id)).size !== providers.length) throw new Error('Duplicate provider IDs');
if (registry.some(p => !categories.includes(p.category))) throw new Error('Unknown category');
const target = new URL('../internal/providers/catalog.json', import.meta.url);
await writeFile(target, JSON.stringify(providers, null, 2) + '\n');
console.log(`Exportados ${providers.length} providers`);

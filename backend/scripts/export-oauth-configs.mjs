// Exporta as configs OAuth reais do registry original para o snapshot Go.
import registry from '../../open-sse/providers/registry/index.js';
import { writeFile } from 'node:fs/promises';
const out = {};
for (const entry of registry) {
  if (!entry.oauth) continue;
  const o = entry.oauth;
  out[entry.id] = {
    scope: o.scope || null,
    extraParams: o.extraParams || {},
    fixedPort: o.fixedPort || null,
    callbackPath: o.callbackPath || null,
    clientId: o.clientId || null,
    clientSecret: o.clientSecret || null,
    tokenUrl: o.tokenUrl || null,
    authorizeUrl: o.authorizeUrl || null,
    scopes: Array.isArray(o.scopes) ? o.scopes : [],
    redirectUri: o.redirectUri || null,
    codeChallengeMethod: o.codeChallengeMethod || null,
  };
}
await writeFile(new URL('../internal/providers/oauth-configs.json', import.meta.url), JSON.stringify(out, null, 2) + '\n');
console.log(`Exportadas ${Object.keys(out).length} configs OAuth`);

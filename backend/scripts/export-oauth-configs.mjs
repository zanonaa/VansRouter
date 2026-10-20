// Exporta as configs OAuth reais do registry original para o snapshot Go.
import registry from '../../open-sse/providers/registry/index.js';
import { GOOGLE_OAUTH_CLIENT, ANTIGRAVITY_OAUTH_CLIENT } from '../../open-sse/providers/shared.js';
import { writeFile } from 'node:fs/promises';
const out = {};
for (const entry of registry) {
  if (!entry.oauth) continue;
  const o = { ...(entry.id === 'gemini-cli' ? GOOGLE_OAUTH_CLIENT : entry.id === 'antigravity' ? ANTIGRAVITY_OAUTH_CLIENT : {}), ...entry.oauth };
  out[entry.id] = {
    providerId: o.providerId || null,
    cliInitUrl: o.cliInitUrl || null,
    cliPollUrl: o.cliPollUrl || null,
    businessLoginUrl: o.businessLoginUrl || null,
    planApiKeyName: o.planApiKeyName || null,
    initiateUrl: o.initiateUrl || null,
    pollUrlBase: o.pollUrlBase || null,
    apiBaseUrl: o.apiBaseUrl || null,
    authorizeDeviceUrl: o.authorizeDeviceUrl || null,
    deviceCodeUrl: o.deviceCodeUrl || null,
    copilotTokenUrl: o.copilotTokenUrl || null,
    userInfoUrl: o.userInfoUrl || null,
    apiVersion: o.apiVersion || null,
    userAgent: o.userAgent || null,
    deviceScope: typeof o.scopes === 'string' ? o.scopes : null,
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

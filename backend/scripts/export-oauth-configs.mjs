// Exporta as configs OAuth reais do registry original para o snapshot Go.
import registry from '../../open-sse/providers/registry/index.js';
import { GOOGLE_OAUTH_CLIENT, ANTIGRAVITY_OAUTH_CLIENT } from '../../open-sse/providers/shared.js';
import { writeFile } from 'node:fs/promises';
import { QODER_LOGIN_URL,QODER_DEVICE_TOKEN_URL,QODER_USERINFO_URL } from '../../open-sse/shared/qoder/constants.js';
const out = {};
for (const entry of registry) {
  if (!entry.oauth) continue;
  const o = { ...(entry.id === 'gemini-cli' ? GOOGLE_OAUTH_CLIENT : entry.id === 'antigravity' ? ANTIGRAVITY_OAUTH_CLIENT : {}), ...entry.oauth };
  out[entry.id] = {
    loadCodeAssistEndpoint: o.loadCodeAssistEndpoint || null,
    onboardUserEndpoint: o.onboardUserEndpoint || null,
    loadCodeAssistUserAgent: o.loadCodeAssistUserAgent || null,
    webAppUrl: o.webAppUrl || null,
    validationUrl: o.validationUrl || null,
    defaultBaseUrl: o.defaultBaseUrl || null,
    authorizeUrlPath: o.authorizeUrlPath || null,
    tokenUrlPath: o.tokenUrlPath || null,
    userInfoUrlPath: o.userInfoUrlPath || null,
    baseUrl: o.baseUrl || null,
    loginCodePath: o.loginCodePath || null,
    loginStatusPath: o.loginStatusPath || null,
    oauthTimeoutMs: o.oauthTimeoutMs || null,
    referrer: o.referrer || null,
    stateUrl: o.stateUrl || null,
    platform: o.platform || null,
    pollInterval: o.pollInterval || null,
    tokenExchangeUrl: o.tokenExchangeUrl || null,
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
Object.assign(out.qoder,{loginUrl:QODER_LOGIN_URL,deviceTokenUrl:QODER_DEVICE_TOKEN_URL,userInfoUrl:QODER_USERINFO_URL});
await writeFile(new URL('../internal/providers/oauth-configs.json', import.meta.url), JSON.stringify(out, null, 2) + '\n');
console.log(`Exportadas ${Object.keys(out).length} configs OAuth`);

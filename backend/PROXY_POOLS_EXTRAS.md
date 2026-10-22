# Proxy pools: fitness e import público

Porta de `open-sse/services/proxyPoolFitness.js` + `src/app/api/proxy-pools/{fitness,fitness/clear-all,[id]/fitness/clear,import-public}/route.js` em `backend/internal/http/handlers/{pool_fitness,pool_fitness_routes,public_proxy_sources,import_public}.go`.

- Serviço de fitness read-through (cache em memória + `proxyPoolFitness` como fonte): `markPoolUnfit/clearPoolUnfit/isPoolFit` (com curinga `provider::*`), `fitPoolIds`, `clearAllPoolUnfit(provider opcional)`, `resetPoolFitness`, `pruneExpired`, `poolFitnessSnapshot` (expirados deletados na leitura).
- Rotas: `GET /api/proxy-pools/fitness` (`{pools}`), `POST /api/proxy-pools/fitness/clear-all` (`{provider?}` → `{ok, provider}`), `POST /api/proxy-pools/{id}/fitness/clear` (`{scope}` obrigatório → 400 `pool id and scope are required`), `POST /api/proxy-pools/import-public`.
- import-public real: fontes `freeproxydb` (API de busca, page_size fresca 100 + ranking client-side success/latency/recent) e `vpslab` (listas ip:port no GitHub raw); validações por source (país/anonymity/httpsOnly/maxSpeed/orderBy) com mensagens literais; dedupe pela key normalizada (http/https com `/` trailing, socks5 sem); criação em `proxyPools` com `failed` contado; 429 propagado; upstream com timeout 20 s e mensagens `HTTP <status>: <corte 200>. <hint de rate limit>`. Caminho de URLs fixo por fonte — sem SSRF.
- Divergência: `URLSearchParams` serializa vírgula como `%2C` idêntico ao Go (`url.Values.Encode`).

## Deploys de relay (cloudflare/deno/vercel)

Porta das rotas `POST /api/proxy-pools/{cloudflare,deno,vercel}-deploy` em `pool_deploys.go`: 
- Cloudflare: PUT multipart do worker (index.js = `RELAY_TARGET_GUARD_SOURCE` + default fetch export), subdomain enable best effort, GET `/workers/subdomain`, `https://{project}.{subdomain}.workers.dev`, 400 sem credenciais/sem subdomínio, erro de upload com `errors[0].message`.
- Deno: POST `/v2/apps` (409 literal), POST `/apps/{id}/deploy` com `main.ts`, poll `/revisions/{id}` a cada 2 s até 60 s, DELETE do app na falha, `https://{project}.{orgSlug}.deno.net`.
- Vercel: POST `/v13/deployments` (3 arquivos), PATCH `/v9/projects/{id}` (ssoProtection null), poll 3 s/120 s, `https://{url}`, 500 em ERROR/CANCELED/timeout.
- Worker/Deno/Edge relay contêm o JS real embutido (fonte do guard idêntica); divergência: nomes fallback base36 (`Date.now().toString(36)` ⇒ strconv FormatInt base 36).

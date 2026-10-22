# Proxy pools: fitness e import público

Porta de `open-sse/services/proxyPoolFitness.js` + `src/app/api/proxy-pools/{fitness,fitness/clear-all,[id]/fitness/clear,import-public}/route.js` em `backend/internal/http/handlers/{pool_fitness,pool_fitness_routes,public_proxy_sources,import_public}.go`.

- Serviço de fitness read-through (cache em memória + `proxyPoolFitness` como fonte): `markPoolUnfit/clearPoolUnfit/isPoolFit` (com curinga `provider::*`), `fitPoolIds`, `clearAllPoolUnfit(provider opcional)`, `resetPoolFitness`, `pruneExpired`, `poolFitnessSnapshot` (expirados deletados na leitura).
- Rotas: `GET /api/proxy-pools/fitness` (`{pools}`), `POST /api/proxy-pools/fitness/clear-all` (`{provider?}` → `{ok, provider}`), `POST /api/proxy-pools/{id}/fitness/clear` (`{scope}` obrigatório → 400 `pool id and scope are required`), `POST /api/proxy-pools/import-public`.
- import-public real: fontes `freeproxydb` (API de busca, page_size fresca 100 + ranking client-side success/latency/recent) e `vpslab` (listas ip:port no GitHub raw); validações por source (país/anonymity/httpsOnly/maxSpeed/orderBy) com mensagens literais; dedupe pela key normalizada (http/https com `/` trailing, socks5 sem); criação em `proxyPools` com `failed` contado; 429 propagado; upstream com timeout 20 s e mensagens `HTTP <status>: <corte 200>. <hint de rate limit>`. Caminho de URLs fixo por fonte — sem SSRF.
- Divergência: `URLSearchParams` serializa vírgula como `%2C` idêntico ao Go (`url.Values.Encode`).

# OAuth xAI (Grok) e proxies loopback codex/xai

Cobre `GET /api/oauth/[provider]/authorize|start-proxy|poll-status|stop-proxy` e `POST .../exchange|manual-code` para xAI, além do trio de proxy para codex e xai, espelhando `providers.js`, `constants/xai.js`, `utils/server.js` e `app/api/oauth/[provider]/[action]/route.js`.

- xai não tem bloco `oauth:` no registry: config fixa come de `constants/xai.js` (clientId da transport, escopo, porta 56121, redirect base64url de 96 bytes) em `providers.GetOAuthConfig("xai")`.
- Discovery `auth.x.ai/.well-known/openid-configuration` com cache de processo; endpoints validados (https, host `x.ai`/`*.x.ai`) e, em qualquer falha, fallback para os valores estáticos como no try/catch original. Texto de divergência: entradas não analisáveis por `url.Parse` usam a mensagem do Go.
- e-mail do `id_token` sem verificação de assinatura (email → preferred_username → sub); `providerSpecificData` guarda apenas `idToken`.
- Proxies loopback reais em 127.0.0.1:1455 (codex) e 56121 (xai), com sessões por state, guard de Origin loopback (rejeita cross-origin com 403 e página "Authentication Failed"), mode A (troca no servidor + persistência) e mode B (302 para a porta do app com query original). Timeout de 5 minutos encerra. `stopCallbackProxy` fecha apenas o listener — equivalente a `server.close()` do Node — para não cortar a resposta em voo (usar `Server.Close()` quebrava a resposta do callback).
- `manual-code` exige sessão registrada; erros limpam sessão e param o proxy como a origem. `start-proxy` sem sessão responde `serverSide:false`.

Testes: authorize (parâmetros nonce/plan/referrer, PKCE S256, fallback), exchange (form-urlencoded público, sem client_secret, mapeamento do id_token), manual-code (session missing/code ausente/sucesso/limpeza), ciclo completo do proxy com retorno HTML, poll-status consumindo sessão, mode B, 404, guard de Origin e stop/start; codex em mode A com `lastRefreshAt`. Sem rede externa; discovery injetado.

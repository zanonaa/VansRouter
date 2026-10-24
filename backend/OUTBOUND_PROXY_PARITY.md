# Proxy de saída

O runtime resolve o proxy por requisição, com prioridade pool > proxy legado da conexão.

- `proxyPoolIds` com estratégias `none`, `fill-first`, `round-robin`, `random` e `smart` (fitness por `provider::modelo`).
- `proxyPoolId` único continua aceito; `__none__` remove a seleção.
- `noProxy` do pool/conexão é aplicado por hostname, incluindo `*` e subdomínios.
- Pools `vercel`, `cloudflare` e `deno` usam relay via `x-relay-target`/`x-relay-path`.
- `strictProxy=true` nunca cai para o IP direto. Sem strict, falha de transporte no proxy repete a requisição direto, como no original.
- Pool inativo com strict falha antes de sair; sem strict, cai para o proxy legado ou direto.
- O proxy global das Settings é lido do ambiente a cada requisição e restaurado do banco no startup, então vale sem reiniciar e após restart.
- `/v1/chat/completions` (OpenAI), `/v1/embeddings` e `/v1/images/generations` usam o mesmo resolvedor.

Testes unitários cobrem pool, noProxy, proxy legado, fallback, strict, pool inativo, rotação e ambiente dinâmico. Validação ao vivo no Dev usou proxies controlados: pool, noProxy, proxy legado, rotação e proxy global foram confirmados por log de `CONNECT`.

Fora do escopo deste transporte: sondas de validação/descoberta de provider continuam diretas, como na origem.

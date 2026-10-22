# Fase 4A — Lote 1: fundação Vite da SPA (2026-10-22)

Primeiro lote da Fase 4A (React SPA sem Next.js), base 8453db8b. Estado: **fundação entregue; migração em andamento**.

## Lote 3 (2026-10-22): smoke de runtime no navegador

- Corrigido shim `router.refresh()`: chamadas `push(url)` + `refresh()` liam a URL antes da navegação pendente e cancelavam o push (bug observado no login); agora revalida a rota corrente após a navegação.
- `define: { "process.env": {} }` no Vite: módulos open-sse leem `process.env` em nível de módulo (ex. `config/grokCli.js`), o que crashava o bundle no navegador.
- `scripts/spa-runtime-smoke.cjs` (playwright-core + Chromium do sistema): sobe Go (`PORT=20128`, `DATA_DIR` descartável, `JWT_SECRET`, `INITIAL_PASSWORD`) e Vite dev (20129, proxy `/api`), e valida no navegador real: raiz→`/masuk`; login→`/dashboard`; layout do dashboard renderiza; deep link `/dashboard/providers` com reload mantém sessão; rota inexistente mostra 404 da SPA. Resultado: `SMOKE_OK`, com as chamadas `/api` visíveis pelo proxy.
- Descoberta operacional: `/api/auth/check` só responde 200 com `JWT_SECRET` definido (`CheckConfigured`) — o smoke documenta essa exigência.

## Lote 2 (2026-10-22): guard de páginas

- `src/spa/authGuard.js` + `AuthGuard.jsx`: decisão de autenticação sempre via `/api/auth/check` (cookie de sessão no backend); lógica pura testada em `tests/unit/spa-auth-guard.test.js` (autoriza 200; nega 401/403; deny-by-default em falha de rede; redirect raiz por autenticação).
- Rotas fiéis ao dashboardGuard: `/` apenas redireciona (autenticado→`/dashboard`, senão→`/masuk`; landing preservada em `/landing`), `/login`→`/masuk` sem renderizar, `/dashboard*` envolvido por `AuthGuard` com tela de verificação, `/masuk` autenticado→`/dashboard` e não autenticado carrega `/api/auth/status`.
- Divergência aceita: um instante de verificação (loading) onde o SSR decidia antes do HTML.


## O que entrou

- `vite.config.js` + `index.html`: build Vite (rolldown/oxc), alias `@`→`src`, alias `open-sse` replicando jsconfig, proxy dev `/api` → `localhost:20128` ( porta interna Go), scripts `dev:spa`/`build:spa`.
- Plugin `jsx-in-js`: o App Router permite JSX em `.js`; todo `.js` de `src/` passa pelo `transformWithOxc` oficial com parser JSX (runtime automatic) antes da cadeia normal.
- Camada `src/compat/`: `next/link`, `next/navigation` (useRouter/usePathname/useSearchParams/useParams/notFound), `next/dynamic` (lazy+Suspense) e `next/image` (`<img>` nativo) reimplementados sobre react-router — nenhum código do pacote next entra no bundle.
- `src/spa/`: entrada, mapa completo das 25 rotas do App Router (mesmos caminhos/parâmetros, lazy por página), layout raiz com Theme+I18n, error boundaries raiz e do dashboard (reaproveita `(dashboard)/error.js`), fontes (Material Symbols woff2 subset local; Inter via Google Fonts).
- Leituras de banco eliminadas do navegador: `/masuk` busca `/api/auth/status` (estado de carregamento preservado); `media-providers/[kind]` busca `/api/providers`, `/api/nodes` (filtro custom-embedding mantido) e `/api/combos`; página endpoint renderiza o client direto (a prop SSR `machineId` nunca foi consumida pelo `EndpointPageClient`).
- Bootstrap de servidor do `app/layout.js` (proxy de saída, bootstrap, captura de console) não é importado pela SPA — schedulers pertencem ao processo Go desde a Fase 3.

## Validação

- `pnpm build:spa`: verde; 75 chunks, ~4 MB, zero módulos Node externalizados (fs/crypto/os/db fora do grafo), zero ocorrências de `/_next`/`next/dist` nos assets.
- `pnpm lint`: 0 erros (warnings pré-existentes mantidos; `dist/` ignorado).
- `pnpm test`: 4004 testes aprovados (375 arquivos), sem regressões.
- `vite preview`: index servido e `/dashboard` com fallback 200.

## Divergências conhecidas (aceitas neste lote)

1. Inter carrega do Google Fonts (`<link>`) em vez de self-host do `next/font`.
2. `next/image` vira `<img>` sem otimização/redimensionamento automático.
3. `router.refresh()` re-navega para a rota atual; não há refetch de Server Components (inexistente em SPA).
4. Fast Refresh não se aplica a `.js` com JSX (plugin pré-transforma para JS puro); HMR desses arquivos faz full reload em dev.
5. `notFound()` lança `Response 404` capturada pelo error boundary raiz (visual próprio de 404).

## Pendências do 4A (próximos lotes)

- Exercitar fluxos reais contra o Go local (login/OIDC, CRUD, streaming) e validar `useSearchParams`/deep links.
- Auditar `NEXT_PUBLIC_*` (hoje apenas BASE_URL/CLOUD_URL) para env Vite.
- Integração por mesma origem, remoção do Next e de `src/app/api` (Fase 4B).

A Fase 4B não foi iniciada. Não há declaração de conclusão da Fase 4A.

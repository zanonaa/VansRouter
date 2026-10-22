# Fase 4A — Lote 1: fundação Vite da SPA (2026-10-22)

Primeiro lote da Fase 4A (React SPA sem Next.js), base 8453db8b. Estado: **fundação entregue; migração em andamento**.

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

- Reproduzir o `dashboardGuard` na SPA (redirect login/dashboard) e o middleware de página.
- Exercitar fluxos reais contra o Go local (login/OIDC, CRUD, streaming) e validar `useSearchParams`/deep links.
- Auditar `NEXT_PUBLIC_*` (hoje apenas BASE_URL/CLOUD_URL) para env Vite.
- Integração por mesma origem, remoção do Next e de `src/app/api` (Fase 4B).

A Fase 4B não foi iniciada. Não há declaração de conclusão da Fase 4A.

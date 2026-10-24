# Fase 4B — Lote 2: remoção do runtime Next.js (2026-10-22)

Estado após este lote: **aplicação sem Next.js** — build Vite + binário Go; composição/tag RC permanecem para as Fases 5/6.

## Gate de verificação (executado antes da remoção)

- Nenhum import real de `src/app/api` fora dele (as três ocorrências em `src/` eram comentários).
- Consumidores do runtime Next identificados: 40 arquivos de teste, scripts de build/deploy, eslint-config-next, custom-server/server.js, middleware (`src/proxy.js` + `src/dashboardGuard.js`).

## Removido

- `src/app/api/**` (104 rotas `next/server`): todos os contratos têm destino Go desde a Fase 3 (192 registrados, 0 pendentes).
- Entradas de execução Next: `custom-server.js`, `server.js`, `next.config.mjs`, `scripts/build.js`, `scripts/start.js`, `scripts/deploy-atomic.cjs`, `scripts/fix-standalone-symlinks.cjs`, `scripts/verify-release-artifact.cjs`, `scripts/package-closure.cjs`, `scripts/preflight-upgrade.cjs`; scripts `dev*/build*/start*` do Next no package.json (`build` agora é `vite build`, `dev` é `vite`).
- Middleware Next: `src/dashboardGuard.js` e `src/proxy.js` — a autenticação real é o middleware Go (deny-by-default `/api/*`, LocalOnly, SettingsGuard) mais o guard de páginas da SPA (lote 2 da 4A).
- Páginas server-only sem consumidor: `layout.js` (next/font), `error.js`, `global-error.js`, `manifest.js`, `masuk/page.js` (SSR), `endpoint/page.js`, `media-providers/[kind]/page.js` — substituídas pelos wrappers da SPA na 4A.
- Dependências: `next`, `@next/env`, `@next/third-parties`, `eslint-config-next`, `@swc/helpers`, `sharp`; entradas correspondentes no `pnpm-workspace.yaml`. `runtime-secrets.cjs` foi mantido (política de segredos independe de Next; integração ao deploy Go na Fase 6).
- `docker/native-deps` (binário better-sqlite3 do runner Node — o runner agora executa o binário Go) e respectiva entrada do `.gitignore`.

## Reescrito para o novo runtime

- `eslint.config.mjs`: base `@eslint/js` com parser JSX para `.js` (comportamento do preset Next), plugin react-hooks, `no-undef` por escopo com globals Node 22/browser, e regras novas do recommended em warning (baseline 0 erros preservado; limpeza incremental).
- `ecosystem.config.cjs`: PM2 executa o binário Go do release (`vansrouter`, `interpreter: none`) com `PORT/DATA_DIR/SPA_DIR`; `readPm2App/readPm2Environment` migradas de `deploy-atomic.cjs` para `scripts/runtime-paths.cjs`.
- `Dockerfile`: três estágios — Vite (node:22-alpine digest-pinada) → Go (`go mod download` + `go.sum`) → runner com binário + `dist` em `SPA_DIR=/app/spa`; entrypoint mantém migração de volume legado via node e `su-exec`. Digest da imagem golang fica para a Fase 6 (exige verificação de registry).
- Testes de contrato ajustados: `production-secrets` (sem launchers Node), `open-package-external` (sem next.config), `release-hardening` (build `vite build`, contratos Docker Go, sem native-deps; scripts livres de next), `server-entrypoint` (release resolve `vansrouter`), `runtime-paths` (PM2 lido de runtime-paths.cjs), `material-symbols-subset` (fonte via @font-face da SPA).

## Testes removidos (contratos do runtime Next, sem par substituto na nova pilha)

`custom-server-h2c`, `custom-server-peer-headers`, `db-webpack-runtime`, `deploy-atomic`, `fix-standalone-symlinks`, `upgrade-preflight`, `dashboard-guard`, `public-hosts`, `aggregator-providers-batch` e os 40 que importavam `src/app/api/**`. A cobertura equivalente dos contratos HTTP vive na suíte Go (Fase 3); regressão visual/navegação é escopo da Fase 5.

## Validação

- vitest: 3597 testes aprovados (328 arquivos); lint 0 erros; `pnpm build` (Vite) verde.
- Smoke `SAME_ORIGIN=1` reexecutado contra o build novo: `SMOKE_OK` (login, dashboard, deep link, CRUD, SSE EventSource na mesma origem).

## Pendências do 4B/6

- Digest da imagem golang no Dockerfile; composição integrada, regressão visual e tag RC (Fases 5/6).
- Política `runtime-secrets` aplicada ao processo Go no deploy; revisão de docs/README com comandos Next remanescentes.

A Fase 5 não foi iniciada.


Primeiro lote da Fase 4B (integração), base 3b8e5e99. Estado: **mesma origem entregue; remoção do Next não iniciada**.

## Lote 1 (2026-10-22): mesma origem no Go

## O que entrou

- `SPA_DIR` em `backend/internal/config` (env, opcional): diretório do build da SPA; carregamento falha rápido se não existir ou não for diretório.
- `router.NewWithSPA`: quando `SPA_DIR` está ativo, o Go entrega a SPA pela mesma origem — `/` e rotas da SPA (`/login`, `/masuk`, `/callback`, `/landing`, `/dashboard/*`) servem `index.html` com `no-store`; assets do build são servidos com `immutable` (hash do Vite) e público sem hash com revalidação de 1h; rotas fora da SPA e arquivos inexistentes continuam 404. Sem `SPA_DIR`, `/` mantém o Hello World anterior (comportamento preservado).
- Proteção contra path traversal (caminho resolvido dentro do build), `Content-Type` garantido para js/css/svg/woff2 e suporte a HEAD via `http.ServeContent`.
- Caminhos de API inexistentes nunca recebem HTML: sem sessão o middleware devolve 401 (deny-by-default); autenticado, 404 — ambos verificados em teste.

## Validação

- Go: suíte completa, `vet`, `-race` e build aprovados no sandbox; novos testes cobrem index nas rotas da SPA, cache dos assets, 404 de API/fora-da-SPA, traversal, HEAD e preservação do modo sem SPA.
- Smoke de navegador em modo `SAME_ORIGIN=1` (`scripts/spa-runtime-smoke.cjs`): Go serve API e SPA na 20128 com o `dist` real — raiz→`/masuk`, login→`/dashboard`, layout, deep link com reload, 404 da SPA, CRUD `/api/keys`, `GET /api/models` e **SSE via EventSource na mesma origem** (`/api/translator/console-logs/stream`, 200 `text/event-stream`). `SMOKE_OK`.
- Isso fecha a pendência da 4A (proxy do dev server Vite prendia SSE); o dev com proxy continua válido para desenvolvimento.

## Pendências do 4B (próximos lotes)

- Remoção das dependências e entradas de execução Next.js (`next dev/build`, custom-server, `src/app/api`) após confirmar ausência de consumidores; gate do plano: verificar antes de remover.
- CORS/aliases públicos e limites de streaming finais; Docker/CI para o build integrado (SPA + Go).
- Regressão visual/navegação e tag RC (Fases 5/6).

A Fase 5 não foi iniciada.

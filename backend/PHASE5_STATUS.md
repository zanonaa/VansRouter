# Fase 5 — regressão visual/navegação e substituições de testes (2026-10-22)

Estado: **verificações da Fase 5 entregues**; composição/tag RC permanecem para a Fase 6. Base e432b159.

## O que entrou

- `tests/unit/spa-routes-map.test.js` — contrato do mapa de rotas: toda `page.js` restente em `src/app` tem rota na SPA, todo import lazy aponta para arquivo existente, `/dashboard/settings/pricing` permanece fora do `DashboardLayout` e `src/spa/` sem páginas duplicadas.
- `scripts/spa-visual-regression.cjs` — varredura na mesma origem (Go + `dist`): para cada uma das **25 rotas** (incluindo dinâmicas `:kind/:id`, `combo/:id` e `callback`), navegação direta + reload com verificação de que nenhuma cai no error boundary (`VansAI could not load` / `404` / `Dashboard error`), nenhuma resposta de API 5xx durante a varredura e screenshots por rota (artefato local, não commitado).
- `scripts/check-spa-secrets.cjs` — varre o bundle (`dist/**`) por chaves sensíveis do `.env.example` com valor e padrões de credencial (`sk-…`, `GOCSPX-…`); placeholders do próprio template são permitidos (a landing documenta `INITIAL_PASSWORD='replace-with-a-strong-password'`).

## Resultados (2026-10-22)

- Varredura visual: `VISUAL_OK` — 25/25 rotas renderizam com deep link e reload; 44 screenshots.
- Segredos: `OK` — 78 arquivos do bundle, 17 chaves sensíveis verificadas, nenhuma ocorrência.
- Smoke de regressão `SAME_ORIGIN=1`: `SMOKE_OK`.
- Gates: vitest 3601 aprovados (329 arquivos), lint 0 erros, build Vite ok.

## Substituições de testes (documentação exigida pela fase)

Os 49 testes removidos na 4B eram contratos do runtime Next.js. Substituições:

- Contratos HTTP das rotas `src/app/api/**` → suíte Go da Fase 3 (192 contratos; inventário `backend/PHASE3_INVENTORY.json`).
- Guard de middleware (`dashboard-guard`, `public-hosts`) → middleware Go (deny-by-default) + `tests/unit/spa-auth-guard.test.js` + guard de páginas (`src/spa/AuthGuard.jsx`).
- Deploy/standalone (`deploy-atomic`, `fix-standalone-symlinks`, `upgrade-preflight`, `custom-server-*`, `db-webpack-runtime`) → contratos do novo runtime em `tests/unit/release-hardening.test.js` e `server-entrypoint.test.js`; política de segredos mantida em `production-secrets.test.js`.
- Descoberta de modelos em rota (`aggregator-providers-batch`) → suíte Go de provider discovery.
- Navegação/renderização (novo nesta fase) → `spa-visual-regression.cjs` + `spa-routes-map.test.js`.

Comparação visual pixel-a-pixel com baseline do app original não é possível (runtime Next removido); os screenshots desta varredura servem de baseline para futuras regressões.

## Pendências (Fase 6)

- Integrar `spa-visual-regression.cjs` e `check-spa-secrets.cjs` ao CI com o build Vite; digest da imagem golang; política `runtime-secrets` no processo Go; composição final e tag RC.
- Regressão visual pixel-a-pixel opcional com baseline fixado a partir desta varredura.

A Fase 6 não foi iniciada.

# Fase 4B — Lote 1: mesma origem no Go (2026-10-22)

Primeiro lote da Fase 4B (integração), base 3b8e5e99. Estado: **mesma origem entregue; remoção do Next não iniciada**.

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

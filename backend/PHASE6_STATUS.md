# Fase 6 — composição final (2026-10-22)

Estado: **composição integrada e validada**; falta apenas a decisão de tag RC. Base 0d21da8e.

## O que entrou

- **Política de segredos no processo Go** (`backend/internal/config`): porteiros do `runtime-secrets.cjs` — em `NODE_ENV=production`, `JWT_SECRET`/`API_KEY_SECRET` com placeholder conhecido abortam o boot (mensagens idênticas às do script Node). Testes cobrem placeholder/forte/fora-de-produção.
- **Migração do volume legado portada para o boot do Go** (`backend/internal/database/legacy_migration.go`): porta fiel de `docker/migrate-legacy-volume.cjs` — cópia sem sobrescrever, validação `PRAGMA integrity_check`, rejeição de sidecar `-wal/-shm`, lock com detecção de dono morto/stale, marker atômico com fsync, backup do destino inválido. Executada antes de abrir o banco; falha aborta o boot. Cinco testes cobrem os caminhos do original (sem fonte, migração válida, preserva destino válido, sidecar/fonte inválida, canônico inválido sem fonte).
- **Runner sem Node**: `docker/migrate-legacy-volume.cjs` e o Node do runner foram removidos; entrypoint apenas `chown` + `su-exec app`. Imagem final **44,5 MB** (alpine puro + binário + dist). Digests pinados com verificação no Docker Hub: `golang:1.25-alpine@sha256:1ae0735f…` e `alpine:3.22@sha256:d406446b…`.
- **CI estendido** (`.github/workflows/ci.yml`): ESLint, `vite build`, `check-spa-secrets.cjs` e smoke estático de composição (Go serve `/dashboard`, asset e 404 de API) antes do build multi-arch Docker; contratos do `release-hardening` mantidos verdes.
- Scripts de smoke/visual ganham `SPA_PORT` configurável.

## Validação

- Docker real: `docker build` multi-stage ok; container sobe com login (`INITIAL_PASSWORD`), `/api/auth/check` 200 autenticado, SPA e asset servidos, `which node` → **SEM_NODE**; migração legada validada no container com volume real montado em `/migration-data` (banco movido, `extra.txt` copiado, marker criado, health 200).
- Gates: suíte Go + `vet` + `-race` + build no sandbox; vitest 3591 aprovados; lint 0 erros; build Vite; `check-spa-secrets` OK; smoke `SAME_ORIGIN` `SMOKE_OK`; regressão visual 25/25 `VISUAL_OK` — tudo com o binário final.
- Contratos ajustados: `database-paths-verification` (migrador Go, `su-exec app`, sem node no Dockerfile); `legacy-volume-migration.test.js` removido (contrato portado para os testes Go do migrador).

## Pendência final

- ~~**Tag RC**~~ — **criada por decisão explícita do usuário em 2026-10-22**: `v1.2.0-rc1` (anotada) aponta para `ff001cc5`, o commit da composição validada (PR #159). Publicada no fork.
- Opcional: fixar baseline pixel-a-pixel a partir dos screenshots atuais; revisão de README/docs com fluxo Docker novo.

## Encerramento da migração

Com a tag RC, as seis fases do plano revisado (seção 14 do MIGRATION_PLAN.md) estão implementadas: backend Go integral (Fase 3), SPA React/Vite sem Next.js nem SSR (4A/4B), regressão visual/navegação com substituições de testes documentadas (5) e composição final integrada em Docker com runner sem Node (6). A imagem multi-arch é construída e publicada pelo CI em pushes para `main` (`ghcr.io/zanonaa/vansrouter`); o compose continua apontando para a imagem oficial (`VANSROUTER_VERSION`).

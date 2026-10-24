# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
ARG APP_VERSION=unknown
ARG VCS_REF=unknown

# Fase 6: imagem integrada Go + SPA (sem Next.js). O runtime Node permanece
# apenas para o PXPIPE do Token Saver, que executa o pacote npm em library mode
# exatamente como a origem (install on-demand + transform por subprocesso).
# Step 1: build da SPA com Vite na plataforma nativa do builder.
FROM --platform=$BUILDPLATFORM node:22-alpine@sha256:0a7108bf6c7bf5de370ffb1a3ed6be93d405b43ff159f681a8d18c0e2bc2e402 AS spa-builder
WORKDIR /app

ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0 \
    PNPM_HOME=/pnpm
ENV PATH="${PNPM_HOME}:${PATH}"
RUN corepack enable && corepack prepare "pnpm@9.15.9" --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/package.json ./frontend/package.json
RUN --mount=type=cache,target=/pnpm/store \
  pnpm install --frozen-lockfile --store-dir=/pnpm/store

COPY frontend ./frontend
RUN pnpm run build && test -f frontend/dist/index.html \
  && grep -q '\.flex{' frontend/dist/assets/*.css \
  && grep -q '\.grid{' frontend/dist/assets/*.css

# Step 2: build do backend Go. Cross-compile nativo: o stage roda na
# plataforma do builder (BUILDPLATFORM) e GOARCH compila para o alvo — arm64
# sai sem emulação QEMU.
# Digest multi-arch do manifest verificado no Docker Hub (Fase 6).
FROM --platform=$BUILDPLATFORM golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS go-builder
ARG TARGETARCH
# A versão calculada no CI precisa entrar no arquivo embutido por
# //go:embed. Sem isso o binário sempre reportava 2.0.0 e o /api/version não
# acompanhava o build publicado.
ARG APP_VERSION=unknown
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
RUN if [ "$APP_VERSION" != "unknown" ]; then printf '%s\n' "$APP_VERSION" > internal/http/handlers/app-version.txt; fi \
  && CGO_ENABLED=0 GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/vansrouter ./cmd/api

# Step 3: runner alpine puro: binário Go + dist. A migração do volume legado
# roda dentro do binário no boot (backend/internal/database/legacy_migration.go).
FROM alpine:3.22@sha256:d406446bc05b82480756ce8f1287964d28768c23955fa9480d2a74a2eae7a654 AS runner
ARG APP_VERSION
ARG VCS_REF
WORKDIR /app

RUN apk add --no-cache su-exec ca-certificates tzdata nodejs npm \
  && node -v && npm -v \
  && addgroup -g 1000 app \
  && adduser -D -u 1000 -G app app \
  && mkdir -p /app/data \
  && chown -R app:app /app/data

COPY --from=go-builder /out/vansrouter /app/vansrouter
COPY --from=spa-builder /app/frontend/dist /app/spa
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /app/vansrouter

ENV PORT=20128 \
    DATA_DIR=/app/data \
    SPA_DIR=/app/spa \
    RELEASE_BUILD_ID=${VCS_REF}

EXPOSE 20128
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/app/vansrouter"]

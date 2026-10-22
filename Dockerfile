# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
ARG APP_VERSION=unknown
ARG VCS_REF=unknown

# Fase 4B: imagem integrada Go + SPA (sem Next.js).
# Step 1: build da SPA com Vite na plataforma nativa do builder.
FROM --platform=$BUILDPLATFORM node:22-alpine@sha256:0a7108bf6c7bf5de370ffb1a3ed6be93d405b43ff159f681a8d18c0e2bc2e402 AS spa-builder
WORKDIR /app

ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=0 \
    PNPM_HOME=/pnpm
ENV PATH="${PNPM_HOME}:${PATH}"
RUN corepack enable && corepack prepare "pnpm@9.15.9" --activate

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
RUN --mount=type=cache,target=/pnpm/store \
  pnpm install --frozen-lockfile --store-dir=/pnpm/store

COPY index.html vite.config.js ./
COPY src ./src
COPY public ./public
RUN pnpm run build && test -f dist/index.html

# Step 2: build do backend Go.
# Nota: digest da imagem golang será pinado na Fase 6 (exige verificação de
# registry); a imagem node do stage 1 já é digest-pinada.
FROM golang:1.25-alpine AS go-builder
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/vansrouter ./cmd/api

# Step 3: runner. Mantém node:22-alpine (digest pinado) apenas para o script de
# migração de volume legado; o processo do app é o binário Go com a SPA em SPA_DIR.
FROM node:22-alpine@sha256:0a7108bf6c7bf5de370ffb1a3ed6be93d405b43ff159f681a8d18c0e2bc2e402 AS runner
ARG APP_VERSION
ARG VCS_REF
WORKDIR /app

RUN apk add --no-cache su-exec ca-certificates tzdata

COPY --from=go-builder /out/vansrouter /app/vansrouter
COPY --from=spa-builder /app/dist /app/spa
COPY docker/migrate-legacy-volume.cjs /usr/local/bin/migrate-legacy-volume.cjs
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh /app/vansrouter

ENV PORT=20128 \
    DATA_DIR=/app/data \
    SPA_DIR=/app/spa

EXPOSE 20128
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/app/vansrouter"]

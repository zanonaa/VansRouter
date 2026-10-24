# VansRouter

Self-hosted web dashboard and API gateway for managing AI providers, models, API keys, combos, quotas, and usage.

[![Docker image](https://img.shields.io/badge/Docker-AMD64%20%7C%20ARM64-blue?logo=docker)](https://github.com/zanonaa/VansRouter/pkgs/container/vansrouter)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## Docker

```bash
docker run -d \
  --name vansrouter \
  -p 20128:20128 \
  -v vansrouter-data:/app/data \
  -e INITIAL_PASSWORD='replace-with-a-strong-password' \
  --restart unless-stopped \
  ghcr.io/zanonaa/vansrouter:1.2.0
```

The image supports **Linux AMD64 and ARM64** under the same tag. Open `http://localhost:20128/masuk` and sign in with the password you set.

## Build from source

Requirements: Go 1.25+, Node.js 22.5+ and pnpm 9.

The repository is split into:

- `backend/`: Go API, gateway, persistence and HTTP server.
- `frontend/`: React/Vite dashboard, public assets, frontend configuration, shared browser catalogs, JavaScript tests and tooling.
- Root: pnpm workspace/lockfile, Docker, CI and project documentation.

```bash
pnpm install --frozen-lockfile
pnpm test
pnpm build
cd backend
go test ./...
SPA_DIR="$(pwd)/../frontend/dist" go run ./cmd/api
```

For frontend development, run `pnpm dev` from the repository root (Vite on port `20129`, API proxy to Go on `20128`). The root pnpm commands delegate to `frontend/`; they also work directly inside that directory.

The app listens on port `20128`. Persistent data is stored under `~/.9router` by default; set `DATA_DIR` to choose another path.

## License

MIT — see [LICENSE](LICENSE).

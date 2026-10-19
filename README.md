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

Requirements: Node.js 22.5+ and pnpm 9.

```bash
pnpm install --frozen-lockfile
pnpm test
pnpm run build
pnpm start
```

The app listens on port `20128`. Persistent data is stored under `~/.9router` by default; set `DATA_DIR` to choose another path.

## License

MIT — see [LICENSE](LICENSE).

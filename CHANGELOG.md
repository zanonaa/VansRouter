# Changelog

## 1.2.0

- Simplified CI to one Linux job that tests, builds the application, and builds Docker images for AMD64 and ARM64.
- Published Docker tags from `main` to GHCR.

## 0.91.70

- Removed the static “Available free tiers” catalog from the Quota page.
- Removed the Skills, CLI Tools, and Remote promotional pages from the fork.
- Fixed authenticated provider model discovery so providers such as TokenHarbor use their saved API key; only explicitly public catalogs are fetched anonymously.

Earlier release history from the upstream project is intentionally omitted from this web-gateway fork.

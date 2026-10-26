# Official Token Saver runtimes

## Delivered

- PXPIPE: npm `pxpipe-proxy@0.15.1`, bundled at build time with npm lockfile integrity verification. `PXPIPE_PACKAGE_ROOT` selects the read-only bundle; dashboard install becomes an availability check, never a runtime download. Native/non-Docker installation pins the same version. Existing start/stop behavior remains.
- Ponytail: optional `official` level embeds the upstream compact `AGENTS.md` without modification, at commit `09c434ab73ca43761c5d8b0c5d341926c2fba710`, with MIT license. Legacy levels remain for compatibility; this does not install client hooks/commands.
- Headroom: official `headroom-ai[proxy]==0.41.0`, separate non-root container with Python dependency hashes. `compose.token-savers.yml` is an optional overlay. No host port; private internal network; telemetry/beacon disabled. Kompress ML disabled to avoid model downloads. Compression remains local via `/v1/compress`; provider credentials are not sent to this endpoint by the adapter.

Run from this repository with your existing application compose file:

```sh
docker compose -f /path/to/application-compose.yml -f compose.token-savers.yml build headroom
docker compose -f /path/to/application-compose.yml -f compose.token-savers.yml up -d headroom vansrouter
```

Compose resolves relative build paths against its first file. Use `--project-directory` pointing to this checkout, or set the overlay build context to the absolute checkout path. The application service must be named `vansrouter`; adapt otherwise. Preserve its existing networks (including its outbound network); do not replace them with only the internal compression network.

Then set `headroomUrl` to `http://headroom:8787` in Token Saver settings and enable Headroom deliberately. External Headroom is managed by Compose, not dashboard start/stop. This overlay is not an automatic production deployment.

## RTK: official CLI via stdin

The gateway now invokes official Apache-2.0 RTK CLI 0.46.0 (`rtk pipe`) on tool-result text before sending the final request. This works with OpenAI Chat Completions and Responses because filtering is independent of the provider endpoint. The Go code only traverses message structures; copied Go filters have been removed. Autodetection and filtering belong entirely to the upstream CLI; unsupported text remains unchanged. No shell commands from messages are executed. Each eligible result has a 2-second timeout and missing/failed CLI leaves the original text with a warning, never a legacy-filter fallback.

Docker builds the official CLI from commit `8dae5a750fe307d1fee105a2ff4a9d1ca54ac791` with archive SHA256 verification and Cargo.lock, targeting each image architecture. This avoids the fixed upstreams and separate BUSL license of RTK Proxy. The CLI license is included in the image. Native installations must install `rtk` on PATH.

Ponytail legacy level keys (`lite/full/ultra`) now map to the exact official compact rules for settings compatibility; their former custom prompt variants have been removed. The UI offers only the official level.

## Validation and limits

Local AMD64 runtime validation: Headroom health 200 (`rust_core=loaded`) and `/v1/compress` 200 with the adapter request shape; PXPIPE official exports and synthetic transform (`below_min_chars`). These are not savings benchmarks. No paid model generation is used. ARM64 runtime execution must be validated separately from Go cross-compilation. Versions are intentionally not `latest`; update manifests and locks together.

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

## RTK: deliberately not replaced yet

The official RTK Proxy currently compresses Anthropic Messages only; OpenAI Chat Completions is forwarded unchanged. Upstreams are process-wide boot settings, not per-request. Inserting it into the generic gateway would risk breaking provider selection, proxy-pool egress, OAuth routes and credential isolation. No undocumented compression API is assumed.

Its BUSL-1.1 additional use grant permits free production use but restricts offering it or derivatives as a paid/hosted product/service. Obtain appropriate permission before distributing an integration for such use. Sources reviewed:

- https://github.com/rtk-ai/rtk/blob/master/proxy/LICENSE
- https://github.com/rtk-ai/rtk/blob/master/docs/guide/proxy/configuration.md

The old Go filters remain, explicitly labeled as internal legacy filters in the UI. They are not the official RTK engine. Installing the RTK CLI in the server cannot intercept commands executed on clients. Full replacement needs a separately designed, licensed integration preserving the gateway's existing egress rules.

## Validation and limits

Local AMD64 runtime validation: Headroom health 200 (`rust_core=loaded`) and `/v1/compress` 200 with the adapter request shape; PXPIPE official exports and synthetic transform (`below_min_chars`). These are not savings benchmarks. No paid model generation is used. ARM64 runtime execution must be validated separately from Go cross-compilation. Versions are intentionally not `latest`; update manifests and locks together.

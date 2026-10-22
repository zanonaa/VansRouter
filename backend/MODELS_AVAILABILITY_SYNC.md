# Modelos: availability e catalog-sync

Porta de `src/app/api/models/{availability,catalog-sync}/route.js` (+ `src/lib/modelCatalog/sync.js`) em `backend/internal/http/handlers/{models_availability,model_catalog_sync}.go`.

- `GET /api/models/availability`: model locks ativos (`modelLock_*` com data futura; corte do prefixo, sem prefixo → `__all`) e fallback `__all`/`unavailable` para conexões com `testStatus:"unavailable"`; `connectionName` = name||email||id; `lastError` null quando ausente. `POST` com `{action:"clearCooldown",provider,model}`: apaga `modelLock_<model>` nos locks ativos para o provider e restaura `testStatus/lastError/lastErrorAt/backoffLevel` quando unavailable; 400 literal `Invalid request`.
- `GET /api/models/catalog-sync`: estado do sync (`running/lastSync/lastError/lastResult/etag/fileVersion/file/url/intervalMs`) + resumo do arquivo (`{syncedAt, models, providers, bytes}`) ou `catalog:null`.
- `POST /api/models/catalog-sync`: sync real contra `https://models.dev/api.json` (60s timeout, `if-none-match` só quando fileVersion==2), build() fiel (aliases 9router→models.dev, baseId, modalities filed por provider local E upstream, tolerância 10% nos deltas), `slim()` para `model-catalog-raw.json`, escrita atômica; 304 → `{status:"unchanged"}`; erro → 503 com `lastError`/`sync in progress`.
- `StartTimer`: sync diário (delay 60s, retry 30min), `MODEL_CATALOG_SYNC=off` desliga, guard com started.
- Divergências: `current`/limites vêm do snapshot consolidado `models-catalog.json` (JS usa `getCapabilitiesForModel` das tabelas hand-written) e entries `contextLength` usa `caps.contextWindow`; install lock `contextLength==0` preservado.
- Pendente: `POST /api/models/test` depende dos endpoints `/api/v1/*` (chat/embeddings/images/stt) e entra no lote do gateway.

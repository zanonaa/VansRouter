# Legacy thinking fixtures

These modules are **test-only reference implementations** from the former JavaScript gateway. They preserve regression cases while the corresponding behavior is verified/ported to Go; they are not imported by `frontend/src`, not bundled by Vite and not runtime frontend utilities.

Do not add production imports of this directory. The UI only consumes format constants and model-name/availability metadata from `src/catalogs/`. Request translation belongs in `backend/`.

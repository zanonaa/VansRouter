# Modelos e desabilitados

GET/PUT /api/models e GET/POST/DELETE /api/models/disabled com HEAD/OPTIONS, guard e persistência kv (scope disabledModels). Catálogo de 1395 modelos exportado do open-sse (scripts/export-models-catalog.mjs) com routedModel e caps; listagem aplica aliases por fullModel, filtra desabilitados por provider+model e recusa alias duplicado.

Validação: suíte completa, vet, build e race aprovados em sandbox sem rede; integração cobre CRUD disabled, alias duplicado, filtro e guard. Runtime Next/Go: 16 requests TCP coincidiram (disabled CRUD, alias aplicado nos dois, conflito de alias, 400s).

Limites: modelos dinâmicos de fetchers (noAuth/passthrough) não são adicionados pelo Go — o gerador Next/aliases cobre catálogo estático; catalog-sync/availability/test, allowedModelsCache e gateway seguem pendentes. Snapshot precisa regeneração quando o registry mudar.

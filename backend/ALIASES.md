# Aliases e modelos customizados

GET/PUT/DELETE /api/models/alias e GET/POST/DELETE /api/models/custom com HEAD/OPTIONS, guard e persistência kv (scopes modelAliases/customModels) — mesma tabela já migrada. Sanitização replica a origem: caps restritas a vision/reasoning booleanas; transport aceito apenas gemini-live em stt; upsert substitui caps inteira e mescla demais campos; bulk ignora duplicatas/ausentes dentro de uma transação.

Validação: suíte completa, vet, build e race aprovados em sandbox sem rede. Diferencial com aliasRepo.js original cobre 15 operações comparando retorno e conteúdo integral da tabela kv após cada passo (ordem de chaves JSON ignorada). Runtime Next/Go comparado por TCP: 23 requests coincidiram (guard, login, erros, set/update/delete alias, sanitize caps/transport llm-stt, upsert, bulk com contagem e delete por query). Scripts/export-aliases-fixture.mjs e scripts/runtime-aliases.py reproduzem.

Dependência de fase futura: allowedModelsCache do gateway Node não é invalidado pelo Go (não há cache Go equivalente ainda); implantação híbrida Node+Go compartilhando DATA_DIR pode servir catálogo de modelos obsoleto até o gateway Go existir. Modelos catalog/availability/disabled/test e o gateway seguem pendentes; este lote não os implementa.

# CRUD de combos

GET/POST /api/combos, GET/PUT/DELETE /api/combos/{id}, HEAD/OPTIONS ligados ao guard existente. Persistência SQLite usa schema já migrado; nenhuma tabela nova.

Implementa nome, duplicata, models/kind/context_length, 201/400/404, update transacional e delete. context_length preserva coerções Number JS comuns (booleanos, strings numéricas e hex/bin/oct), limite positivo inteiro até 2.000.000. Payload do combo é direto, sem envelope no item, como na origem.

Validação: suíte completa, vet, build e race aprovados em sandbox sem rede. Integração SQLite cobre CRUD, proteção, duplicata, contexto e métodos. Diferencial com combosRepo.js original cobre 8 operações e 17 valores de validação de contexto; timestamps/UUID normalizados.

Dependência de runtime: resetComboRotation e invalidateAllowedModelsCache da origem atuam no gateway JavaScript. O Go ainda não tem executor/cache de seleção de combos; portanto não há cache Go a invalidar. Não opera sobre cache do processo Node, nem permite implantação híbrida com writes Go/reads Node sem considerar essa diferença. Quando o gateway Go for portado, suas mutações devem invalidar os caches de seleção e rotação. CRUD não é implementação de fallback/round-robin/streaming.

Runtime Next versus Go comparado em sandbox sem rede: 14 requests TCP incluindo guard, login, erros, criação, duplicata, get/update/list, HEAD/OPTIONS, delete e 404. Status/payload coincidiram normalizando UUID/timestamps. Cópia Next descartável do mesmo código de combos; dados fictícios separados. Scripts/runtime-combos.py reproduz o fluxo no ambiente preparado. Os testes não representam cobertura universal de tipos arbitrários ou concorrência entre processos. Este lote não conclui a Fase 3.

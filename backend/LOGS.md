# Leitura de logs e detalhes

GET /api/usage/logs e /api/usage/request-logs retornam os últimos 200 logs derivados de usageHistory, ordem id DESC, nomes de connections com fallback ao ID curto. Preserva fallback [] da origem em falha de leitura. Nenhum writer novo de gateway neste lote.

GET /api/usage/request-details pagina/filtra JSON armazenado com pagination, intervalo inclusivo normalizado, parseInt dos parâmetros e redação de request/providerRequest/providerResponse/response. GET /api/usage/request-details/{id} mantém detalhe integral sob guard, 404 Not found e erro Failed to fetch detail. HEAD/OPTIONS implementados.

Validação: suíte completa, vet, build e race aprovados em sandbox. Runtime Next/Go com 12 requests TCP e banco populado coincide: guard/login, logs com NULL/fallback de tokens, JSON redigido, detalhe integral, paginação, datas inclusivas e ausência. Scripts/runtime-logs.py é reproduzível no namespace preparado, diretórios descartáveis e dados fictícios.

Não implementa captura/bufferização de novas requisições, gravação pelo gateway, resets de quota ou /usage/{connectionId}. Datas ISO/date-only e parseInt comuns testados; formatos de data dependentes do parser JS não têm cobertura universal. Não é conclusão da migração.

# Pricing

GET/PATCH/DELETE /api/pricing com HEAD/OPTIONS e guard. Defaults derivados de PROVIDER_PRICING, embutidos no binário. PATCH valida campos/valores e persiste overrides por modelo em transação; retorna apenas overrides como na origem. GET mescla campos dos defaults. DELETE reseta modelo/provider/tudo e retorna configuração mesclada. GET_DEFAULTS da origem não é verbo HTTP exportado válido, portanto não cria /defaults fictício.

Validação: suíte completa, vet, build e race aprovados. Runtime Next/Go: 14 requests coincidiram (guard, defaults completos, inputs inválidos, PATCH acumulativo, GET e resets). Script runtime-pricing.py, dados descartáveis e rede isolada.

Go lê overrides em cada GET, sem cache de 5s do Node: resultados ficam mais recentes para alterações externas, sem promessa de paridade de cache cross-process. Snapshot precisa regeneração se pricing.js mudar. Resolução de preço por pattern/model e contabilização do gateway não fazem parte deste CRUD.

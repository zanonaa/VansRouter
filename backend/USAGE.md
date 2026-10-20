# Uso (histórico, gráfico, providers)

GET /api/usage/history (filtros provider/model/startDate/endDate, DESC, limite 500), GET /api/usage/chart (today/24h em 24 buckets horários "HH:MM"; 7d/30d/60d/all diários a partir de usageDaily com dias zerados e labels "Oct 14") e GET /api/usage/providers (DISTINCT em requestDetails + nome do nó quando existir). HEAD/OPTIONS e guard.

Validação: suíte completa, vet, build e race aprovados em sandbox sem rede; integração com usageHistory+usageDaily+requestDetails cobre filtros, buckets, hoje com dia zerado, invalid period e guard. Runtime Next/Go: 8 requests TCP coincidiram em estado vazio, incluindo formato de labels/buckets que exigiu duas correções (janela 7d=hoje-6 e buckets horários de hoje). Scripts/runtime-usage.py reproduz.

Pendências: /usage/logs, /usage/[connectionId], resets de quota, request-details detalhado, request-logs e gravação de uso pelo gateway seguem fora deste lote. Chart diário lê usageDaily (resumo), não usageHistory — paridade mantida com a origem; se o gateway Go gravar só history, o chart precisa do fluxo de resumo diário equivalente.

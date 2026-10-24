# Usage stream e conferência de logs

GET /api/usage/stream com SSE inicial, atualização via notificações/Track, keepalive 25s e remoção de assinatura ao cancelar. Estado real de requests pendentes com timeout 60s, nomes de conexão, requests recentes deduplicados e erro de provider por 10s. Track e Notify serão ligados ao gateway quando ele for portado; não há execução simulada de requests.

GET /api/usage/logs e request-logs já tinham handlers: registros agora são literais para a auditoria estática.

Suíte, vet, build e race completos aprovados em sandbox. Teste cobre início/fim/erro e proteção contra contador negativo.

Limitações: histórico consultado pelo método existente carrega todo o histórico antes de limitar a 50; origem usa ring buffer de 50. Snapshot não mantém cache de nomes 30s nem debounce 200ms. SSE ainda depende da integração futura do gateway para receber eventos de requests executados. A rota HEAD automática da origem não foi adicionada pois handler streaming precisa de tratamento específico.

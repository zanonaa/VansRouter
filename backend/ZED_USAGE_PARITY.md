# Usage Zed

Consulta `/client/users/me` com autorização `userId token` e system ID opcional. Normaliza planos, previsões, pedidos hospedados, trial, reset e faturas vencidas. Limite zero de modelos indica cobrança por token e não cria quota fictícia.

Teste injetado verifica header, percentual, omissão da quota e ausência de user ID. Suíte, vet, race e build aprovados em sandbox.

Pendências: Grok CLI JSON/gRPC, proxy/refresh e homologação diferencial. Não encerra a Fase 3.

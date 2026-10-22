# Usage Grok CLI

Consulta billing e usuário com headers do CLI, desembrulha valores `{val}` e normaliza mensal, on-demand, prepaid e percentual. Promoção esgotada não é tratada como ilimitada. Sem quotas JSON, decodifica o frame gRPC de créditos; frame truncado ou inválido retorna erro explícito.

Testes cobrem headers, percentuais e protobuf sem rede externa. Suíte, vet, race e build aprovados em sandbox após limpeza de caches temporários identificados.

Pendências: envelopes adicionais de créditos, plano JWT, proxy/refresh e homologação diferencial. Não encerra a Fase 3.

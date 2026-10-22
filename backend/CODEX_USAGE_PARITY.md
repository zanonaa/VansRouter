# Usage Codex

Consulta wham/usage e normaliza janelas normal/review/spark, aliases de payload, limites atingidos, créditos de reset e datas numéricas/ISO. Percentuais são limitados a 0–100; zero explícito não cai no campo alternativo. API indisponível retorna a mensagem original.

Testes cobrem todos os tipos de janela, porcentagens/datas em strings, campos alternativos, endpoint e erro 429. Suíte/vet/race/build aprovados em sandbox, com teste dirigido reexecutado após ajuste nullish.

Pendências: proxy/refresh/cache no wrapper e diferenças de datas arbitrárias do Date JS; demais usage providers continuam pendentes. Fase 3 aberta.

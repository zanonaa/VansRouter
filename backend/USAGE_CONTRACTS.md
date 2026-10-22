# Usage contracts

GET /api/usage/stats consulta usageHistory real e valida períodos. GET /api/usage/{connectionId} retorna 404 para conexão ausente, mensagem para conexão inelegível e 501 explícito quando o executor do provider ainda não foi portado. Claude reset e Codex credits fazem chamadas reais com o token salvo. Suíte, vet, build e race aprovados em sandbox.

# Usage normalizado

DeepSeek converte balance_infos em quotas de crédito, inclusive strings numéricas, e distingue saldo insuficiente. Ollama converte razões 0–1 em percentuais, limita excesso, consulta formato de plano e calcula reset mensal gratuito. Erros e autenticação mantêm mensagens da origem.

Testes locais verificam moeda, percentuais, plano e reset. Suíte, vet, race e build aprovados em sandbox.

Pendências: chamada /api/me, Vercel e demais providers, proxy/refresh, normalização de datas e comparação diferencial. Não encerra Fase 3.

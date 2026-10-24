# Usage Claude

Consulta OAuth usage com cedar_ember, normaliza janelas de sessão/semana/modelo, scoped limits e grants de reset. Fallback settings/organization em falha OAuth. Cache local por hash de token com TTL de cinco minutos, deduplicação de consulta, cópia de resultados, force e cooldown de três minutos após 429; preserva dados anteriores quando atualização não traz quotas.

Testes injetados verificam quotas/grants, fallback, concorrência, cache/force e cooldown. Suíte, vet, race e build aprovados em sandbox. Cache é por instância; timeout/proxy/refresh no wrapper, validação diferencial e demais usage providers continuam pendentes. Não encerra Fase 3.

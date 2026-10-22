# Usage Gemini e Antigravity

Gemini resolve projectId da conexão ou loadCodeAssist, consulta buckets e normaliza frações. Antigravity distingue 401, 403 e conta inelegível, omite modelos no plano gratuito, filtra modelos internos e adiciona quota semanal com cache e deduplicação. Falha semanal não descarta quotas já obtidas.

Testes injetados cobrem projeto, buckets, plano gratuito, filtro e concorrência. Suíte, vet, race e build aprovados em sandbox.

Pendências: plataforma dinâmica do User-Agent, proxy/refresh e homologação diferencial. Não encerra a Fase 3.

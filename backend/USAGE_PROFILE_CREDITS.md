# Usage: perfil Ollama e créditos Vercel

Ollama consulta POST /api/me best-effort e usa plano/CreatedAt na normalização; falha do perfil não descarta quotas. Vercel consulta /v1/credits com a chave da conexão e normaliza saldo/uso conforme o denominador de USD 5 da origem.

Testes usam transporte injetado para perfil disponível/indisponível e créditos em strings. Suíte, vet, race e build aprovados em sandbox. Removidos somente binários temporários identificados após quota excedida na linkedição.

Pendências: detalhe textual de erros upstream, proxy/refresh e demais providers. Não encerra Fase 3.

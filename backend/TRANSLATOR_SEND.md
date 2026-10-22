# Translator send (OpenAI-compatível)

POST /api/translator/send seleciona conexão ativa, executa transporte OpenAI/custom compatível e faz streaming real da resposta. Erros upstream preservam status, mensagem e detalhes. Providers com formato especial retornam 501 explícito.

Validação: suíte, vet, race e build em sandbox. Teste injeta transporte e verifica endpoint, autenticação, streaming e provider não suportado.

Pendências: refresh automático 401/403, transformações específicas, retries transitórios, proxy de ambiente e formatos não OpenAI não portados. Não equivale integralmente ao executor JS.

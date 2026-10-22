# Renovação de credenciais do translator

Após 401/403, translator/send renova credenciais nos protocolos Claude/Codex (JSON), Google e Qwen (form). Persiste accessToken, refreshToken rotacionado, idToken e expiração antes de repetir a requisição uma única vez. Qwen preserva metadata ao atualizar resourceUrl. Refresh usa proxy/relay configurado e timeout de 30 segundos. API key separada não é sobrescrita pelo bearer OAuth.

Testes verificam preservação do corpo, rotação persistida, uma única repetição mesmo após segundo 401 e ausência de retry quando a persistência falha. Suíte, vet, race e build aprovados em sandbox.

Pendências: deduplicação concorrente do refresh e classificação de invalid_grant, demais protocolos de refresh, reavaliação de URL Qwen após resourceUrl mudar e integração aos executores especiais. Fase 3 permanece aberta.

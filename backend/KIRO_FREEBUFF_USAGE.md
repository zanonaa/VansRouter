# Usage Kiro e Freebuff

Kiro tenta CodeWhisperer GET, POST e Q GET, preservando headers de API key e IdP externo. API key não recebe profile padrão. Falhas de autenticação têm mensagens específicas por método. Freebuff consulta somente GET da sessão, preserva uso fracionário e distingue região bloqueada, banimento e ausência de sessão.

Testes injetados cobrem fallback, ausência de profile e GET exclusivo. Suíte, vet, race e build aprovados em sandbox.

Pendências: texto bruto exato dos erros Kiro, proxy/refresh e homologação diferencial. Não encerra a Fase 3.

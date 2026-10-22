# Limpeza de proxy gerenciado

Settings limpa variáveis gerenciadas quando valores são esvaziados, remove o marcador quando não há mais configuração, preserva ambiente externo e rejeita esquemas/caracteres recusados pela origem. Aplicação é serializada. Testes isolam e restauram as variáveis de ambiente. Suíte, vet, race e build aprovados em sandbox.

Alterar ambiente não reconfigura automaticamente o cache ProxyFromEnvironment do Go; integração dinâmica do transporte e scheduler/rotação continuam pendentes. Não encerra Fase 3.

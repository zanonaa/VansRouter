# Kiro: importações e fluxo social

Portadas quatro rotas: POST import, POST import-cli-proxy, GET social-authorize e POST social-exchange. Registro específico precede o roteador OAuth dinâmico.

Import valida refresh token no endpoint social ou regional AWS OIDC com clientId/clientSecret. CLIProxyAPI normaliza external_idp, valida HTTPS e hostname Microsoft permitido, campos obrigatórios e expiração. Social inicia device authorization e classifica pending/slow_down/erros/sucesso; persiste ou atualiza conexão por identidade sem comparar tokens. ProfileArn isolado não une contas sem identificador de conta.

Hooks: cliente HTTP existente do OAuthEngine; ListConnections e UpdateConnection ligados ao banco no main.

Validação em sandbox: suíte completa, vet, build e suíte completa com race. A primeira execução race falhou por quota de disco na linkedição; após remover cinco binários temporários de lotes recentes, a repetição -p=1 passou.

Limitações conhecidas: datas explícitas CLIProxyAPI aceitam RFC3339/RFC3339Nano, não toda a gramática permissiva de JavaScript Date; mensagens do parser JSON em import/import-cli-proxy são próprias do Go. HTTP usa o cliente existente sem novo timeout, como a origem. Não foram usados tokens reais nem serviços externos nos testes.

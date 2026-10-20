# Continuação após merge autorizado do PR #20

PR #20 integrado em migration/go-backend (5ddaa485). Pendências de PR20_REVIEW.md não foram declaradas resolvidas pelo merge.

## Corrigido

Backup pré-schema somente quando versão, colunas ou índices exigem alteração. Inicialização com schema já compatível não cria backup nem atualiza metadados de migration. Versão futura é recusada antes de gerar backup. Backups existentes não são apagados automaticamente.

Testes verificam boot repetido sem cópias adicionais, backup durante upgrade e preservação das ACLs e tabelas desconhecidas. Suíte completa, vet, build e race aprovados em sandbox sem rede.

## Em aberto

Importação JSON legada segue recusada explicitamente, sem ignorar arquivos. Importação transacional, validação diferencial HTTP/TCP, endpoints de conectividade e demais diferenças descritas na revisão continuam pendentes. Esta entrega não conclui a Fase 3 nem autoriza corte de produção.

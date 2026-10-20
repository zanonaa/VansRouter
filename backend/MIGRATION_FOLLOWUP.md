# Continuação após merge autorizado do PR #20

PR #20 integrado em migration/go-backend (5ddaa485). Pendências de PR20_REVIEW.md não foram declaradas resolvidas pelo merge.

## Corrigido

Backup pré-schema somente quando versão, colunas ou índices exigem alteração. Inicialização com schema já compatível não cria backup nem atualiza metadados de migration. Versão futura é recusada antes de gerar backup. Backups existentes não são apagados automaticamente.

Testes verificam boot repetido sem cópias adicionais, backup durante upgrade e preservação das ACLs e tabelas desconhecidas. Suíte completa, vet, build e race aprovados em sandbox sem rede.

## Em aberto

Importação JSON legada implementada para db.json, usage.json, disabledModels.json e request-details.json: cópia prévia dos arquivos, transação para todos os dados, conferência de quantidade nas coleções principais e marcador após commit. Registro interno goLegacyImported evita duplicar history quando o marcador de arquivo falha após commit. Originais não são removidos. JSON inválido e banco existente com dados sem prova de importação são recusados, diferentemente da tolerância da origem, por segurança; esse comportamento exige aprovação de compatibilidade. Instâncias Node/Go não devem migrar o mesmo DATA_DIR simultaneamente.

Testes cobrem importação de settings/connections/nodes/API keys/combos/aliases/history, preservação do original, retry sem duplicação e rollback por IDs duplicados. Cobertura específica de dailySummary, customModels, pricing, disabled e request-details ainda precisa ser ampliada; implementação não equivale a homologação completa. Validação diferencial HTTP/TCP, endpoints de conectividade e demais diferenças descritas na revisão continuam pendentes. Esta entrega não conclui a Fase 3 nem autoriza corte de produção.

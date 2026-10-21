# Correção do contrato de transferência do banco

Substitui export/import SQL cru do PR #35 pelo payload lógico da origem: settings objeto, connections/nodes/pools com extras descompactados, fitness/API keys/combos e modelAliases/customModels/mitmAlias/pricing. Import limpa só essas coleções; histórico, detalhes, tabelas desconhecidas, scopes alheios e _meta permanecem. settings_revision incrementa após import, numa única transação.

Reautenticação usa hash persistido, não INITIAL_PASSWORD após definição da senha. Header CLI só dispensa senha quando o token é validado, não pela presença de header. Testes verificam negativas, rollback após remoções, preservação de histórico/disabled/tabela desconhecida e rejeição da senha inicial após senha persistida.

Suíte completa/vet/build/race aprovados em sandbox sem rede. Ordenação do export de listas usa os repos Go (a origem lê sem ORDER BY); não promete ordem byte a byte. Assim como a origem, export/import lógico omite ACL de API keys. Efeito applyOutboundProxyEnv do processo Node ainda não existe no Go e depende do transporte/gateway; não declarar esse ponto concluído. Esta correção não fecha Fase 3.

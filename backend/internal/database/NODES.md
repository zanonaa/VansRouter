# Persistência de providerNodes — lote parcial

`Nodes` porta `src/lib/db/repos/nodesRepo.js`: listagem filtrada por tipo, consulta por ID, criação/upsert, atualização por mesclagem transacional e exclusão com retorno do registro removido. Campos extras ficam no JSON `data`; colunas estruturadas prevalecem na leitura. Timestamps seguem ISO UTC com milissegundos.

O pacote não está ligado a novas rotas HTTP, não cria tabelas e não executa migrations. Schema ausente retorna erro. Testes usam banco descartável com a definição de tabela da origem.

Ainda faltam: contratos HTTP de provider-nodes, catálogo/normalização de providers, connectionsRepo (deduplicação OAuth/API key, prioridades e estado de saúde) e proxyPoolsRepo. Nenhum desses blocos deve ser declarado concluído por esta entrega.

Correção do acompanhamento: rotação de segredo JWT não é requisito de paridade identificado no backend original; não será acrescentada sem requisito específico. Limpeza global do limiter também não existe na origem e é melhoria opcional, não bloqueio da migração. Isso não elimina as divergências e pendências já documentadas em `../auth/README.md`, nem equivale a uma auditoria de autenticação aprovada.

Validação deste lote: suíte completa, `go vet`, build e `go test -race` aprovados em sandbox sem rede. Testes cobrem CRUD no SQLite, filtro, registros ausentes, mesclagem/preservação de extras e falha com schema ausente. Sem comparação diferencial Node/Go completa e sem novos testes HTTP, pois nenhuma rota nova foi registrada.

# Persistência de providerNodes — lote parcial

`Nodes` porta `src/lib/db/repos/nodesRepo.js`: listagem filtrada por tipo, consulta por ID, criação/upsert, atualização por mesclagem transacional e exclusão com retorno do registro removido. Campos extras ficam no JSON `data`; colunas estruturadas prevalecem na leitura. Timestamps seguem ISO UTC com milissegundos.

O pacote está ligado a GET/POST /api/provider-nodes, além de HEAD/OPTIONS, sob o middleware de autenticação existente. Não cria tabelas e não executa migrations: schema ausente retorna erro (inclusive em instalações novas enquanto migrations não forem portadas). Testes usam banco descartável com a definição de tabela da origem.

Ainda faltam: PUT/DELETE de provider-nodes (propagam alterações/exclusões às connections), rota de validação, catálogo/normalização de providers, connectionsRepo (deduplicação OAuth/API key, prioridades e estado de saúde) e proxyPoolsRepo. Nenhum desses blocos deve ser declarado concluído por esta entrega.

Correção do acompanhamento: rotação de segredo JWT não é requisito de paridade identificado no backend original; não será acrescentada sem requisito específico. Limpeza global do limiter também não existe na origem e é melhoria opcional, não bloqueio da migração. Isso não elimina as divergências e pendências já documentadas em `../auth/README.md`, nem equivale a uma auditoria de autenticação aprovada.

Validação deste lote: suíte completa, `go vet`, build e `go test -race` aprovados em sandbox sem rede. Testes cobrem CRUD no SQLite, filtro, registros ausentes, mesclagem/preservação de extras e falha com schema ausente. Testes HTTP pelo roteador com SQLite descartável cobrem 401, validações 400/500, criação 201 dos três tipos, prefixos de ID, normalização de URLs, listagem, HEAD e OPTIONS. Sem comparação diferencial Node/Go completa nem validação com servidor TCP nesta atualização.

# POST /api/providers/bulk

Registrado sob o guard existente, com OPTIONS. Valida 1–500 itens, normaliza provider/nome/apiKey, usa prioridade inteira ou 1, recusa provider inválido/campos ausentes (400), duplicata provider:nome (409) e nó compatível ausente (404). Enriquece metadados de nós antes de chamar a persistência transacional; resposta 201 contém results sem credenciais.

A lista de providers permitidos é intencionalmente diferente do POST individual: bulk não usa authModes para aceitar providers OAuth dual-auth, reproduzindo a origem.

Testes pelo roteador com SQLite descartável cobrem lote vazio, campos ausentes, duplicatas, nó ausente e criação/upsert. Suíte completa, vet, build e race aprovados em sandbox sem rede. Persistência já tem testes de rollback do lote.

Limites: mensagens de exceções internas são fixas, enquanto a origem devolve error.message; tipos arbitrários de providerSpecificData (arrays/strings) não possuem paridade diferencial homologada. Sem benchmark de lote máximo ou teste TCP nesta atualização. Migrations e demais rotas de teste/modelos/gateway permanecem pendentes. Não representa conclusão da Fase 3.

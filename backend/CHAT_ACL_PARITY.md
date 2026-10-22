# ACL do chat

Chat usa requireApiKey, diferencia chave ausente/inválida e impede envio quando allowedKinds não permite llm ou allowedProviders não permite o provider resolvido. Modo local com requireApiKey=false funciona sem chave. CORS aplicado ao fluxo.

Teste verifica que nenhum request upstream ocorre em rejeições e que chamadas permitidas/mode local executam. Suíte, vet, race e build aprovados em sandbox.

Não encerra Fase 3: trusted internal, allowlist dinâmica de modelos, combos, conversões de requisição/resposta e demais endpoints ainda precisam de paridade.

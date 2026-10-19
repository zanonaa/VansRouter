# Catálogo público de providers

`catalog.json` é gerado do registry local `open-sse/providers/registry/index.js` por `node backend/scripts/export-provider-catalog.mjs` (executar na raiz). O Go incorpora esse snapshot; não depende de Node em runtime. Alterações do registry exigem regeneração e revisão do diff.

Exporta os 160 providers das categorias free/freeTier/oauth/apikey/webCookie na ordem usada por AI_PROVIDERS. Inclui somente metadados necessários ao CRUD: ID, aliases, nome, categoria, authModes, noAuth e serviceKinds. Não exporta transport, tokens ou configuração OAuth. O nome usado por GET /api/providers permanece o ID porque a origem consulta def.display?.name após buildProviderEntry espalhar display; não corrigimos esse comportamento silenciosamente.

`GET /api/providers` e HEAD estão registrados sob o guard e removem apiKey/accessToken/refreshToken/idToken das connections, enriquecendo nomes compatíveis e devolvendo aliasMap/providers. POST/PUT/DELETE e OPTIONS da rota ainda não foram portados. Snapshot não inclui modelos/executores e não representa migração do gateway.

Validação: suíte completa, vet, build e race aprovados. Testes cobrem contagem/IDs/arrays, cópia imutável, normalização, aliases e listagem pelo roteador com SQLite verificando omissão de apiKey. Comparação diferencial completa e regeneração automática no CI seguem pendentes.

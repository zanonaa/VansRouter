# Xquik, Ollama e GLM

Xquik valida queryType, envia cursor/idioma e normaliza posts, mídia, cursor e créditos por resultado. Ollama envia país/idioma e conteúdo textual. GLM usa envelope MCP e decodifica JSON interno, com fallback para results/items/lista. JSON inválido do GLM não é mascarado como sucesso vazio.

Testes cobrem URL, autenticação, recusa, paginação e extração sem rede externa. Suíte, vet, race e build aprovados em sandbox após limpeza de caches temporários identificados.

Pendências: ID dinâmico do GLM, Perplexity sem searchConfig no snapshot, overrides/proxy e homologação diferencial. Não encerra a Fase 3.

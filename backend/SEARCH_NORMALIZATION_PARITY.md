# Normalização de busca

Serper, Brave, Tavily e SearxNG usam builders da origem e devolvem resultados unificados com title/url/display_url/snippet/position/score/content/metadata/citation, usage, metrics e total. Tavily aplica topic, país e domínio incluído/excluído. Limites usam configuração default/max. Providers sem normalizador retornam erro explícito, não envelope bruto.

Testes verificam URL/display, total, score limitado, conteúdo completo, filtros e ausência de chamada externa. Suíte, vet, race e build aprovados em sandbox.

Pendências: Exa, Perplexity, Google PSE, Linkup, SearchAPI, You.com, TinyFish, Xquik e demais; opções avançadas, paginação, ACL/fallback/chat search e comparação diferencial. Não encerra Fase 3.

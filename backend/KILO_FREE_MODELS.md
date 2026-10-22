# Kilo: modelos gratuitos

GET/HEAD/OPTIONS /api/providers/kilo/free-models, com fetch real de https://api.kilo.ai/api/gateway/models (timeout 10s), filtro isFree === true, contexto default 0, cache de uma hora e fallback stale com warning. Sem cache, falhas retornam 502 e models: []. Cliente HTTP injetável; testes sem rede cobrem filtro/cache/stale.

Suíte completa, vet, build e race passaram em sandbox. Primeira tentativa race sem espaço na linkedição; removidos apenas binários temporários de validações anteriores e repetição aprovada. Mensagens de rede/parser podem usar o texto do Go, diferente do runtime JS; HTTP não-2xx mantém o texto literal Kilo API returned <status>.

Demais extras de providers continuam pendentes; este documento não representa encerramento do bloco nem da Fase 3.

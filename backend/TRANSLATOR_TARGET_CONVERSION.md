# Conversão de destino e plano compartilhado

Etapa 3 converte OpenAI intermediário para Claude/Gemini com múltiplos turnos, system/developer, ferramentas, chamadas de ferramentas, resultados Claude, imagens inline e parâmetros de geração. Não altera o mapa de origem. Blocos ou mapeamentos não suportados retornam erro em vez de desaparecer. Resolve conexão ativa e monta URL/autenticação usando o mesmo translatorPlan do envio; providers Claude mantêm seus endpoints, Gemini streaming preserva alt=sse.

Testes verificam histórico completo, system/tools, limite de tokens, endpoint GLM e API key, recusa de blocos desconhecidos; regressões de envio/refresh passaram. Suíte, vet, race e build aprovados em sandbox.

Pendências: conversão reversa da etapa 2, demais formatos e blocos, tool results Gemini, respostas e SSE traduzidos, aliases/dinâmicos e quirks por provider. Não encerra Fase 3.

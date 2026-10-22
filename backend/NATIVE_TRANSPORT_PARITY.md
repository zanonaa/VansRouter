# Correção do envio de payloads nativos

- translator/send preserva mensagens, ferramentas, instruções e opções do corpo já traduzido; não inventa texto nem reduz o histórico Gemini à última mensagem.
- Gemini streaming mantém alt=sse ao adicionar a chave.
- Providers Claude usam o endpoint do próprio registry, sem redirecionamento indevido à Anthropic.
- Autenticação distingue descritores apiKey/oauth do registry.
- O snapshot inclui metadados de todos os transportes; OpenAITransport continua restrito ao formato OpenAI. URL disponível não comprova suporte ao protocolo.

Testes verificam corpo completo, múltiplos turnos, endpoint GLM, autenticação Claude OAuth e Gemini streaming. Suíte, vet, race e build aprovados em sandbox.

A Fase 3 permanece aberta. Estes testes não certificam conversão entre formatos, respostas/streams traduzidos, refresh, combos, todos os executores de mídia/usage nem equivalência integral com Node.

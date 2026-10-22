# Contratos restantes da Fase 3

Implementa os 13 endpoints que ainda respondiam 501:

- Busca: Serper, Brave, Tavily e SearxNG, com sanitização, credencial e timeout.
- Fetch: Firecrawl, Tavily, Exa, TinyFish e Jina, com bloqueio de URL local/privada.
- Vídeo: gera, edita, estende e consulta usando a configuração real e devolve o cabeçalho da conexão.
- Áudio: fala e transcrição OpenAI com multipart e resposta original.
- Gemini nativo: encaminha generate/stream para a API oficial com a chave da conexão.
- Chat, mensagens e Responses: normaliza conteúdo OpenAI e usa o executor com streaming.

Validação: suíte, vet, race e build. Inventário: 192 contratos, zero sem registro e zero com implementação pendente.

Limites que permanecem: combos, rotação completa, refresh, todos os formatos de busca/STT/TTS e tradutores especiais não foram reproduzidos. Esses caminhos retornam erro explícito ou usam o fluxo OpenAI correspondente, sem sucesso simulado.

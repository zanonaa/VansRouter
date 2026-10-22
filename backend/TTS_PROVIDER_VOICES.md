# Vozes TTS por provider (deepgram/elevenlabs/inworld/minimax)

Porta de `src/app/api/media-providers/tts/{deepgram,elevenlabs,inworld,minimax}/voices/route.js` em `backend/internal/http/handlers/tts_provider_voices.go`, registradas como rotas GET/HEAD/OPTIONS independentes do bloco media.

- Contratos preservados: chave da primeira conexão ativa (400 com a mensagem literal `No <Provedor> connection found`; minimax usa o provider minúsculo), endpoints/verbos/auth headers originais (Token para Deepgram, Basic para Inworld, Bearer + body `{voice_type}` para MiniMax `minimax.io`/`minimaxi.com`), 502 com `prefixo API <status>: <texto||Failed>` e `base_resp.status_code/status_msg` do MiniMax.
- Agrupamento identical: `{languages, byLang}` ordenado por nome, `?lang=` devolve `{voices}` do grupo (vazio quando não há); dedupe por id; Deepgram usa `languages` ou sufixo do `canonical_name`, Inworld defasa para `en`, MiniMax infere prefixo antes de `_` (sem `_` → `Custom`, que fica por último) e concatena ` · <grupo>` fora de `system_voice`; ElevenLabs primary `labels.language` + `verified_languages` (reaproveita `free_users_allowed` do voice base) com `category:"premade"` ou `is_owner===true`.
- Cache de 24h do `fetchElevenLabsVoices` (open-sse) mantido por apiKey.
- Divergências registradas: ordenação `localeCompare` do JS vira comparação byte a byte; a ordem das chaves do objeto `byLang` em JSON é alfabética no Go (JS preserva inserção); nomes de idioma vêm do snapshot `voice-metadata`/`Intl.DisplayNames` já adotado no PR #62.
- Testes com transporte `RoundTrip` injetado gravando a requisição de saída (sem rede externa) e store de conexões falso; nada de stub de sucesso nas respostas (erros 400/502 e base_resp != 0 cobertos).

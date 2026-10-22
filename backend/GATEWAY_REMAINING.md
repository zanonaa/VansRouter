# Vozes, tokens e registro completo do gateway

- GET /api/v1/audio/voices reutiliza os handlers internos em processo, aplica alias do catálogo, filtro de idioma, CORS e erros upstream. Inclui leitura real de vozes locais por `say` ou PowerShell, com lista vazia somente quando o comando falha.
- OPTIONS de chat, embeddings e imagens agora estão registrados com CORS.
- POST /api/translator/translate implementa a etapa 1: provider, modelo, formato de origem e formato de destino. Etapas 2 e 3 retornam 501 explícito porque o conversor ainda não foi portado.
- POST /api/v1/messages/count_tokens reproduz a estimativa Anthropic por caracteres.
- Os 13 endpoints cujo executor/tradutor ainda não existe foram registrados em `Gateway.NotPorted`, com 401 sem chave e 501 explícito. Nenhum deles finge sucesso.

Validação: suíte, vet, race e build em sandbox.

Pendências reais: tradução das etapas 2/3 e executores de chat, Ollama, Claude, Responses, busca, fetch, áudio, vídeo e Gemini nativo. A Fase 3 não está encerrada.

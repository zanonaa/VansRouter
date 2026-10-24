# Executor OpenAI no gateway

POST /api/v1/chat/completions autentica a chave, resolve `alias/model`, seleciona a conexão ativa e faz streaming pelo transporte OpenAI quando os formatos de origem e destino são OpenAI. Formatos diferentes retornam 501 explícito, sem conversão simulada.

Validação: suíte, vet, race e build. Teste injeta transporte e verifica URL, token e rejeição de formato Claude.

Pendências: Responses, Claude, Gemini, Ollama, busca, fetch, áudio, vídeo e tradutores das etapas 2/3. Fase 3 permanece aberta.

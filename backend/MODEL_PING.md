# Ping de modelo pelo gateway local

POST /api/models/test substitui 501: envia requisição interna por kind (chat/embedding/image/STT), usa primeira chave ativa + token CLI, timeout 30s, valida o conteúdo (não basta status 200), fornece latência/status/erro e nota reasoning-only. STT envia WAV PCM 16kHz mono de 250ms.

Suíte, vet, build e race aprovados em sandbox. Testes com RoundTripper verificam resposta vazia/sucesso e estrutura WAV.

Pendências: unwrap de envelope ClinePass não portado, env MODEL_TEST_TIMEOUT_MS ainda não aplicado, corte de texto usa bytes em vez de UTF-16, o endereço local depende da porta configurada e não da porta alocada quando configurada como 0. Executores do gateway continuam limitados; o resultado de ping reflete essa limitação, não simula suporte.

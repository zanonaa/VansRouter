# MiniMax e PlayHT TTS

MiniMax usa payload T2A com áudio hexadecimal, valida base_resp/status_code mesmo em HTTP 200, decodifica bytes e respeita formato retornado. PlayHT divide usuário/chave e monta voice_engine/voice/output_format. Testes com transporte injetado verificam hex válido/inválido, erro de aplicação e headers. Suíte, vet, race e build aprovados em sandbox.

Modelos/vozes e defaults ainda precisam de resolução integral pelo catálogo. Demais adapters e integração avançada permanecem pendentes; Fase 3 aberta.

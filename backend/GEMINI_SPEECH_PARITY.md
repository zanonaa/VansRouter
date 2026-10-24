# Gemini TTS

Resolve modelos TTS do snapshot e voz padrão, envia AUDIO/speechConfig, prefixa prompt conforme idioma sem sobrescrever instrução de estilo existente e converte PCM mono 16-bit 24 kHz em WAV. Responde binário ou JSON base64. Ausência de áudio reporta finishReason/blockReason.

Testes verificam endpoint/auth, prompt, voz, RIFF/WAV e bytes PCM, além de erro de segurança. Suíte, vet, race e build aprovados em sandbox.

Pendências: catálogo atualizado dinamicamente, proxies/fallback/refresh e demais adapters. Fase 3 aberta.

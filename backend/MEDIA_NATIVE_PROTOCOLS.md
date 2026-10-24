# Protocolos nativos de mídia

STT Gemini envia áudio inline base64 com prompt/idioma e concatena texto; HuggingFace envia áudio bruto preservando model org/id; Deepgram normaliza para {text}; AssemblyAI executa upload/submissão/polling com prazo e cancelamento. MIME deriva do nome quando multipart anuncia octet-stream.

TTS ElevenLabs, Inworld, Cartesia, Fish Audio, Coqui/Tortoise e OpenAI têm builders específicos; áudio pode ser binário ou JSON/base64 conforme query response_format. Erros upstream e áudio ElevenLabs vazio são recusados. Testes de transporte com dados fictícios cobrem endpoints, auth, áudio e respostas; suíte/vet/race/build aprovados em sandbox.

Pendências: Gemini Live, TTS Gemini/Edge/Google/local/AWS/PlayHT/MiniMax/Xiaomi/selfhosted, seleção exata de modelos/vozes do registry, allowlist, combos/fallback/proxy/refresh e fixtures diferenciais. Não encerra Fase 3.

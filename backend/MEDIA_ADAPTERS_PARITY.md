# Adapters iniciais de mídia

TTS/STT usam configurações exportadas do registry, ACL de kind/provider e credenciais. OpenAI/Groq preservam multipart e opções; Deepgram envia texto ou áudio bruto com Token auth, modelo e idioma. Timeout de processamento é 300 segundos. Formatos ainda sem adapter retornam 501 explícito.

Suíte, vet, race e build aprovados em sandbox. Pendências: ElevenLabs, Gemini, AssemblyAI, HuggingFace e demais formatos, além de shaping específico das respostas. Não encerra Fase 3.

# Google Translate TTS

Obtém token f.sid/build do HTML, mantém cache serializado de 11 minutos, monta batchexecute jQ1olc com limpeza de texto e idioma e decodifica base64 da resposta RPC. Retorna binário ou JSON como a origem. Testes usam transporte injetado e validam cache, payload e falha de token; suíte/vet/race/build aprovados em sandbox.

Observação de origem: selfhosted-tts declara openai-speech, mas o dispatcher JS presente não contém esse formato. Não foi introduzido um comportamento novo silencioso para essa inconsistência. Pendências: Edge/local/AWS/Xiaomi e integração de proxy/fallback; Fase 3 aberta.

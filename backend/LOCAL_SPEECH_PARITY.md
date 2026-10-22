# Síntese local

Porta a sequência da origem say → ffmpeg → leitura MP3, com argumentos separados e sem shell. Diretório temporário exclusivo é removido em sucesso/falha; contexto limita a execução a 300 segundos. Responde binário ou JSON base64.

Testes substituem execução por fixture e verificam argumentos literais, ordem e limpeza, sem executar comandos do sistema. Suíte/vet/race/build aprovados em sandbox. Depende de say e ffmpeg instalados, como a origem; não inventa áudio quando ausentes. Demais lacunas mantêm Fase 3 aberta.

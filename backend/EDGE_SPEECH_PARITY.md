# Edge TTS

Obtém token e cookie do Bing, mantém cache serializado de cinco minutos, monta SSML escapado e faz uma nova tentativa após 429 ou 403. Áudio menor que 1024 bytes é recusado. Resposta pode ser binária ou JSON.

Testes injetados verificam retry, escape, gênero e áudio curto. Suíte, vet, race e build aprovados em sandbox.

Pendências: AWS Polly, cache de vozes, proxy e homologação diferencial. Não encerra a Fase 3.

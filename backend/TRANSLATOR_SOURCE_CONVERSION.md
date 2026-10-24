# Conversão de origem

Etapa 2 agora converte Claude, Gemini e Responses para OpenAI intermediário: system, turnos, texto/imagens inline, chamadas e resultados de ferramenta e parâmetros de geração. Responses aceita input textual ou array. Entradas não suportadas geram erro em vez de serem descartadas; origem é copiada antes da conversão.

Testes verificam conversas e ferramentas completas, imagens, parâmetros e recusa de blocos desconhecidos. Suíte, vet, race e build aprovados em sandbox.

Pendências: reasoning/thinking, demais blocos e formatos, IDs sintéticos Gemini repetidos, ferramentas Responses achatadas e integração ao gateway/respostas/SSE. Não encerra Fase 3.

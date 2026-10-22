# Encerramento de SSE

Conversão diferencia finalização protocolar de EOF truncado. Sem message_stop/[DONE]/finishReason conforme o protocolo, EOF gera stream_conversion_error, não sucesso. Erro do provider não produz [DONE]. Índices Claude de blocos são mapeados para índices compactos de ferramentas OpenAI.

Testes cobrem truncamento, erro upstream, texto antes de ferramenta e argumentos incrementais. Suíte, vet, race e build aprovados em sandbox. Ainda há pendências de SSE Responses, usage/reasoning e demais protocolos; Fase 3 permanece aberta.

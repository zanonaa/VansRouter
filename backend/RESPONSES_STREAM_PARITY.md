# Responses SSE a partir de Chat Completions

Implementa response.created/in_progress, itens e partes adicionados, deltas/done de texto e argumentos de ferramenta, output_item.done e response.completed/incomplete. Eventos têm sequência crescente. Terminal espera DONE depois do trailer usage e repete os itens completos em response.output, sem perder a resposta para clientes que só leem o terminal. Limite de tokens vira incomplete. Conteúdo acumulado para o terminal tem limite de 16 MiB.

Testes cobrem deltas, argumentos fragmentados, usage tardio, output completo, sequência e limite de tokens. Suíte, vet, race e build aprovados em sandbox.

Pendências: reasoning, providers nativos Responses/Codex, conversão via Claude/Gemini, múltiplas choices, compact e campos adicionais do protocolo. Não encerra Fase 3.

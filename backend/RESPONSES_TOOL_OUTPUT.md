# Saída de ferramentas Responses

Conversão JSON preserva texto e cada tool_call como function_call, com call_id, nome e argumentos. JSON inválido é recusado. Resposta truncada permanece incomplete.

Teste cobre saída mista e argumentos inválidos. Suíte, vet, race e build aprovados em sandbox.

Pendências: chamadas personalizadas, streaming restante, refresh/proxy e homologação diferencial. Não encerra a Fase 3.

# Conversão do gateway

Gateway identifica o protocolo pelo endpoint, converte origem para intermediário e intermediário para destino, preservando corpos nativos quando origem=destino. Respostas JSON são traduzidas entre OpenAI/Claude/Gemini e clientes OpenAI/Claude/Responses nos caminhos implementados. Limite de conversão JSON de 16 MiB.

SSE incremental: Claude/Gemini para cliente OpenAI e OpenAI para cliente Claude; parser aceita fronteiras arbitrárias e CRLF, escreve sincronamente e faz flush, suporta texto e ferramentas incrementais. Blocos/protocolos sem conversão geram erro explícito. Testes cobrem integração request/response, tools/usage, frames de três bytes, argumentos de tool e erro de JSON.

Suíte, vet, race e build aprovados em sandbox. Pendências: Responses SSE, demais pares/protocolos, reasoning, uso completo em streaming, limite de memória eficiente, semântica de interrupção/EOF, multimodal e homologação diferencial. Fase 3 permanece aberta.

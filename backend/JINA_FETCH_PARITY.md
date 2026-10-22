# Fetch Jina

Jina agora envia POST JSON `{url}` ao endpoint base, com Bearer quando há credencial, em vez de anexar a URL ao caminho. Provider sem adapter retorna erro explícito. Título e texto continuam normalizados.

Teste usa transporte injetado e IP público literal, sem rede externa. Suíte, vet, race e build aprovados em sandbox.

Pendências: Ollama fetch sem configuração no snapshot, mensagens detalhadas de erro upstream, timeouts por provider e homologação diferencial. Não encerra a Fase 3.

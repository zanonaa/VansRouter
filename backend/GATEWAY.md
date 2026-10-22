# Gateway inicial

POST /api/v1/chat/completions, embeddings, images/generations e GET /api/v1/models autenticam API key ativa e usam a conexão OpenAI ativa para encaminhar a requisição real. Sem chave retorna 401; sem conexão retorna 404; falha upstream retorna 502. Suíte, vet, build e race aprovados em sandbox.

# Provider validate

POST /api/providers/validate com sondas reais para OpenAI, Anthropic e Gemini. Status 401 invalida a chave; erros de rede retornam 502; provedores sem contrato local retornam 501 e valid=false, sem sucesso simulado. Testes usam transporte injetado. Suíte, vet, build e race aprovados em sandbox após limpeza de binários temporários.

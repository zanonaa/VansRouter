# Normalização de fetch

Firecrawl/Tavily/Exa/TinyFish extraem o conteúdo correto dos envelopes; Jina analisa título do texto. Resposta contém title, content, metadata, usage, metrics e links quando presentes. Aplica max_characters e contagem UTF-16. Corrige Exa ids/text/x-api-key, Tavily bearer/extract_depth e Jina POST JSON.

Testes locais de payloads verificam extração, shape, títulos, limite, comprimento e erros. Suíte, vet, race e build aprovados em sandbox.

Pendências: status exato de falhas TinyFish, truncamento em par substituto Unicode, latências separadas, Ollama, auth/ACL/fallback completo e comparação diferencial HTTP. Fase 3 continua aberta.

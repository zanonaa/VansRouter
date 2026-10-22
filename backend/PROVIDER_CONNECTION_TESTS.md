# Testes de conexão individual e lote (parciais)

POST /api/providers/{id}/test e /api/providers/test-batch substituem handlers 501. Sondas HTTP reais usam configuração de descoberta do registry e endpoints OAuth read-only de Google/GitHub/Qoder/GitLab; suporte a proxy legado/pool/relay; resultado persiste testStatus/lastError/lastErrorAt. Lote filtra conexões ativas por modo, executa em paralelo e mantém ordem dos resultados/summary.

Validação: suíte completa, vet, build e race em sandbox. Testes com transporte injetado verificam token/sucesso/401 e persistência; lote vazio e modos inválidos.

Pendências: renovação, overrides, classificação completa dos providers, probes específicos Codex/Grok/Claude/Kiro/Cursor, verificação prévia do proxy e diagnóstico não portados. Não considerar estes contratos equivalentes integralmente. Requisições de teste precisam de timeout por provider antes de exposição definitiva.

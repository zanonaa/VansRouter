# Combos no runtime

Chat resolve combos por nome, valida allowedCombos, aplica fallback/round-robin sticky, verifica ACL de provider/kind por candidato e mantém streaming da primeira resposta bem-sucedida sem expor erros de candidatos anteriores. Rotação sincronizada, reset por nome em update/delete e reset global em settings substituem o no-op anterior.

Testes verificam 429→sucesso sem vazamento, sequência sticky, ACL e distribuição concorrente. Suíte, vet, race e build aprovados em sandbox.

Pendências: combos aninhados/entradas estruturadas, estratégias avançadas, classificação exata de erros, filas/timeouts e circuit/cooldown integrado. Não encerra Fase 3.

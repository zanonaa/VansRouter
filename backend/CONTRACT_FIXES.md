# Correções após revisão dos lotes modelos/uso

Corrige divergências introduzidas pelos PRs #24/#25:

- POST disabled mescla IDs sem duplicação em transação, em vez de substituir a lista; habilitar o último ID remove a chave.
- Histórico usa ordem de inserção id ASC sem limite artificial de 500, tokens JSON e apiKeyMasked, nunca apiKey crua. Datas normalizadas e endDate inclusivo; data inválida causa o erro HTTP original.

Validação: suíte completa, vet, build e race aprovados em sandbox. Runtime Next/Go com 13 requests populadas confirma chamadas consecutivas disabled, remoção da chave, máscara longa/curta, ordem por inserção, datas e erros. Script runtime-contract-fixes.py usa SQLite e DATA_DIRs descartáveis, sem rede externa.

Fora do escopo desta correção: tipos de IDs não string ainda são descartados pelo handler disabled, diferentemente do Set JavaScript; fetchers dinâmicos do catálogo ainda não migrados. Endpoints de disponibilidade/sync/test e gateway continuam pendentes. Não declarar conclusão da migração.

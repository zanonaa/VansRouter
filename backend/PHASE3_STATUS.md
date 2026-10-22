# Fase 3 — aberta

A Fase 3 NÃO está concluída. Declarações anteriores de fechamento, inclusive títulos de PRs e PHASE3_COMPLETE.md, não comprovam paridade e estão substituídas por este status.

O inventário cruza registro literal com PHASE3_GAPS.json. `partial-known-gaps` identifica contratos com lacunas conhecidas; `registered-needs-contract-review` não significa concluído. Lacunas não são exaustivas. A ausência de Pending/501 não é critério de saída.

PRs #104–#117 corrigiram perda de payloads, autenticação de transporte, vínculo de conta de vídeo, refresh e concorrência, transcrição multipart, ACL, conversões de origem/destino/JSON/SSE, Responses SSE, truncamento, DNS, proxy e combos. Cada lote passou em suíte/vet/race/build no sandbox. Isso não equivale a homologação diferencial integral.

Pendente: protocolos de executores especiais, campos e pares de conversão restantes, mídia por provider, normalização de busca/fetch/usage, catálogo dinâmico e allowlist, estratégias avançadas de contas/combos/proxy/circuitos, refresh completo, quota auto-ping e homologação local comparativa com a origem. Detalhes em PHASE3_GAPS.json e notas de cada lote.

Fase 4 não iniciada. Encerrar somente conforme PHASE3_PROGRESS.md: todos os contratos implementados ou excluídos por decisão explícita do usuário, sem sucesso fictício, testes diferenciais/runtime/streaming/OAuth e desvios aprovados.

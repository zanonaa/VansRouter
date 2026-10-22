# Streaming de reasoning

Pensamento Gemini vira reasoning_content e não é misturado à resposta. Responses abre item/resumo, emite deltas e fecha o resumo antes de texto ou ferramenta. O item final preserva summary_text.

Testes verificam separação, ordem e fechamento. Suíte, vet, race e build aprovados em sandbox após limpeza do cache temporário identificado.

Pendências: assinaturas de pensamento, imagens inline, envelopes adicionais e homologação diferencial. Não encerra a Fase 3.

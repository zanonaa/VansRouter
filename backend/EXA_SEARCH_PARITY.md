# Busca Exa

Builder valida tipos, categorias, país, domínios, datas, schema, contents, stream e combinações incompatíveis antes da chamada. `numResults` tem prioridade sobre `max_results`; news usa categoria news. A resposta normaliza highlights/texto, score limitado, conteúdo e provider_raw, além de requestId, tipo resolvido, custo e output. O custo reportado substitui o custo padrão.

Testes cobrem payload válido, recusas e normalização sem rede externa. Suíte, vet, race e build aprovados em sandbox.

Pendências: stream SSE, paginação local, datas extremas, overrides/proxy e homologação diferencial. Não encerra a Fase 3.

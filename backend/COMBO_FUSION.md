# Combo fusion

A estratégia fusion consulta o painel em paralelo, sem ferramentas, extrai texto e envia um prompt de juiz quando há ao menos duas respostas. Uma resposta é devolvida diretamente. Nenhuma resposta válida retorna 503.

Quorum, tolerância e prazo seguem os defaults da origem. O painel não é enviado ao cliente antes da decisão. Testes cobrem extração, histórico e prompt. Suíte, vet, race e build aprovados em sandbox.

Pendências: cancelamento imediato dos painéis após o prazo, streaming do juiz e homologação diferencial. Não encerra a Fase 3.

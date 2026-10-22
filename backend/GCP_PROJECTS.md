# Projetos GCP por conexão

GET /api/providers/{id}/gcp-projects substitui 501. Preserva 404 conexão ausente, 400 provider incompatível, 401 sem token e resposta de erro Google com status/texto original. Renova token OAuth Google expirado, persiste a rotação e mapeia projects para id/name. Descoberta usa proxy legado, pool HTTP ou relay selecionado; noProxy considera hosts e sufixos.

Validação: suíte completa, vet, build e race em sandbox; transporte injetado testa renovação/persistência/mapeamento e respostas de validação.

Pendências de equivalência: renovação ainda usa transporte direto; resolução não aplica rotação smart/round-robin de pools nem todas as regras noProxy; refresh irrecuperável não tem classificação específica. Esta rota não está certificada como equivalente integralmente até esses casos receberem fixtures.

# Usage GLM, MiniMax e Kimi

GLM escolhe endpoint internacional ou China e normaliza limites por unidade. MiniMax percorre URLs de fallback, reconhece chave inválida, percentuais M-series e janelas de cinco horas e sete dias. Kimi prioriza API key, usa headers OAuth quando necessário e distingue autenticação expirada de falta de permissão.

Testes injetados cobrem região, fallback, percentual e planos. Suíte, vet, race e build aprovados em sandbox.

Pendências: limites adicionais Kimi, versão dinâmica dos headers, proxy/refresh e homologação diferencial. Não encerra a Fase 3.

# Quota auto-ping

Scheduler de um minuto ligado ao ciclo de vida do servidor; consulta configurações a cada tick e só considera contas OAuth ativas com opt-in explícito. Claude verifica reset/último ping; Codex requer janela deslizante, respeita intervalo mínimo e quota bloqueadora semanal. Falhas têm cooldown de 15 minutos. Persistência do reset pingado só ocorre após upstream aceito e corpo drenado. Refresh configurado e ping respeitam proxy/relay. Shutdown cancela e aguarda o scheduler antes de fechar DB.

Testes usam dependências/transporte injetados, sem envio pago ou rede externa: opt-in, quotas, reset, cooldown e resposta interrompida. Suíte/vet/race/build aprovados em sandbox; teste/build reexecutados após ajuste de shutdown.

Pendências: ligação imediata por PATCH em vez de próximo tick, proxy na leitura usage, startup de tokens sem expiresAt, semântica completa de executor Codex/Claude e homologação diferencial. Não encerra Fase 3.

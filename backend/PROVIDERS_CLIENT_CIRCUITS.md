# Conferência de contratos: providers/client e circuit-breakers

- GET /api/providers/client: elegibilidade por features usage/usageApikey do registry, filtro de provider/atividade, ordem/prioridade, paginação limitada a 500, whitelist de campos e máscara de nomes com aparência de token. Backfill Codex uma vez por instância com retry após falha.
- GET /api/providers/circuit-breakers e POST /api/providers/circuit-breakers/{name}/reset: registro em memória compartilhável com o futuro gateway, estados CLOSED/DEGRADED/OPEN/HALF_OPEN, falhas por janela, backoff e histórico. Sem breaker registrado o snapshot é vazio; reset desconhecido tem sucesso como na origem, sem criar breaker.
- Três auto-importadores já existentes agora usam registros literais para que o inventário estático os reconheça. Isso não altera seu comportamento.

Validação: suíte completa, vet, build e race em sandbox. Testes novos: whitelist sem tokens/clientSecret, paginação e transições/recovery/reset.

Pendências de equivalência: ordenação usa comparação Unicode de Go, não localeCompare do Node; máscara usa bytes em vez de UTF-16 para o prefixo. Lista de features é snapshot do registry. CircuitRegistry.Get ainda não atualiza opções de um breaker existente; integração do gateway deverá completá-la. Reset usa SettingsGuard existente (aceita token CLI também); origem possui check adicional por sessão ou requireLogin=false. Essas divergências não constituem fechamento integral da Fase 3.

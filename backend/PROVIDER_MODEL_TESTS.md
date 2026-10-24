# Teste dos modelos de uma conexão

POST /api/providers/{id}/test-models resolve provider/alias, prefixo de node compatível e modelos por snapshot; nodes compatíveis sem catálogo tentam descoberta interna. Primeiro ping termina antes do fan-out; resultados preservam ordem e incluem modelId/name e resultado real do gateway.

Validação: suíte, vet, race e build em bwrap. Teste com transporte injetado cobre descoberta custom e dois pings. Quota de disco na primeira linkedição race resolvida removendo cinco binários temporários recentes identificados.

Pendências: snapshot pode incluir modelos media adicionais em relação ao getProviderModels original; lista dinâmica e executores ainda parciais. A rota depende das limitações documentadas em MODEL_PING.md e não certifica a equivalência completa do gateway.

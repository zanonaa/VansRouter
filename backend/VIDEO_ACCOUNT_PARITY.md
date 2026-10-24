# Correção de vínculo de conta em vídeo

Polling exige x-connection-id, verifica conta ativa, capacidade de vídeo e consistência do provider. Não seleciona a primeira conta para um job pertencente a outra. Criação preserva bytes multipart e Content-Type, encaminha Idempotency-Key e não repete POST após 500 ou erro de rede. JSON tem somente o prefixo do modelo removido. Aplica requireApiKey e ACL de kind/provider.

Testes injetados verificam duas contas, ausência de pin, divergência de provider, bytes multipart/idempotência e ausência de repetição em 500. Suíte, vet, race e build aprovados em sandbox.

Ainda não certifica equivalência integral: adapters de vídeo específicos, allowlist de modelos, refresh e classificação de cooldown/fallback continuam pendentes. Fase 3 permanece aberta.

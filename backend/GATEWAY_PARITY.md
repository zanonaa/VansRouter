# Correção de contratos previamente incompletos

- GET models deixa de encaminhar o catálogo global para OpenAI; lista snapshot de providers ativos/noAuth com requireApiKey e ACL de provider/kind (null=irrestrito, []=bloqueado).
- GET models/info e models/{kind}: metadados gerados do registry por export-model-info.mjs, erros de parâmetro/kind e CORS. Snapshot embutido permite consumo sem Node no runtime.
- Seleção de conexão usa database.Connections.List para preservar tratamento de JSON e colunas.
- Codex POST credits agora usa /consume, request id gerado no servidor e payload/shape de respostas reset/no_credit. GET normaliza contagem/datas e envia headers Codex.
- Claude reset valida grant id, resolve organização via oauth/profile, usa reset_rate_limits com program/grant_id/request_id e normaliza conflitos.
- Inventário diferencia pending-implementation de registrado. Registro 501 não é conclusão.

Validação: suíte completa, vet, build e race em bwrap; testes de ACL vazia, auth, metadados e transporte injetado dos resets.

Pendências: catálogo dinâmico/custom/combos/disabled e capacidades completas não estão integrados; resets ainda não renovam tokens nem resolvem proxies/cache; User-Agent Claude ainda precisa do snapshot de configuração. Chat/embeddings/imagens ainda têm o encaminhamento limitado anterior. Datas numéricas/exatas devem receber fixtures diferenciais. Não há declaração de encerramento da Fase 3.

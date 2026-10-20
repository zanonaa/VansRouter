# Validação de conectividade

Implementados POST /api/provider-nodes/validate e POST /api/proxy-pools/{id}/test, com OPTIONS sob o guard existente.

Nós: /models com fallback /chat/completions, Anthropic com headers e normalização /v1, embeddings com dimensão, timeout real cancelando requisição. Chamadas remotas bloqueiam destinos privados, fixam DNS ao IP validado e não seguem redirects; chamadas locais permitem nós self-hosted. Transporte não herda proxy do ambiente.

Pools: teste HTTP(S) por proxy com GET aos três destinos HTTPS da origem; relays usam x-relay-target/path da origem. Atualiza testStatus/isActive/lastTestedAt/lastError conforme resultado. Nenhum endpoint devolve sucesso fictício.

Testes somente contra loopback no namespace isolado: upstreams simulados, proxy HTTP, túnel CONNECT com TLS e CA fictícia verificada, relay e persistência do resultado. Nenhum proxy/IdP público foi acessado.

Diferenças explícitas: mensagens de erro de transporte não são mensagens específicas do Undici/Node; são saneadas e determinísticas. Redirects de validação remota são recusados e DNS fixado (a origem verifica DNS mas usa fetch separado); limite de leitura é 4 MiB no validator. Exponentes de IPv6 bloqueados incluem multicast. Essas restrições são intencionais, não paridade byte a byte. Produção usa certificados TLS padrão; CA de teste só é injetada no teste CONNECT.

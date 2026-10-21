# Muse device flow

GET /api/oauth/muse/device-code e POST /api/oauth/muse/poll usam endpoints do snapshot com x-api-version 1.0.0, formulário sem PKCE no upstream, pending/slow_down mesmo HTTP 400. O verifier permanece exigido no poll pelo roteador original.

Após autorizar a conta Meta, POST https://api.meta.ai/muse-code/key com onboard:true emite a Model API key. Até três tentativas em 429/5xx, esperas de 5/10s canceláveis; redirects recusados. Assinatura inativa, pagamento necessário, JSON inválido e ausência de api_key falham sem persistência. accessToken da connection é a Model API key, não o token da conta; oauthAccessToken fica em providerSpecificData conforme a origem, sem exposição na resposta.

Testes Route/loopback cobrem pending, inatividade, retries, expiração nula, normalização de email, erros da emissão e redirect sem encaminhar token. Suíte completa/vet/build/race aprovados em sandbox sem rede externa. Nenhuma conta Meta ou emissão pública real foi usada. Não representa fechamento da Fase 3 nem migração do executor Muse.

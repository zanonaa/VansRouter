# Executor do free tier do OpenCode Zen (2026-10-23)

O teste real dos combos (PR #161) mostrou `oc/*` respondendo 403 `FreeTierError`: o upstream exige um contrato de cliente próprio que o gateway não reproduzia. Este lote porta o executor original (`open-sse/executors/opencode.js`, do usuário) para o Go; o contrato foi confirmado de forma independente pelo plugin [tcatarina/cpa-ocfree](https://github.com/tcatarina/cpa-ocfree).

## Contrato implementado (`opencode_free.go` + `opencode_free_send.go`)

- **IDs canônicos** `ses_`/`msg_` + 12 hex + 14 base62 (26 após o prefixo); o hex da sessão carrega `~(ms<<12)` — bit 47 do campo em 1. IDs canônicos do cliente são reaproveitados.
- **Headers de identidade**: `User-Agent: opencode/1.18.31`, `x-opencode-client` (default `desktop`), `x-opencode-project: global`, session/request.
- **Fingerprint do quarteto** `bash/glob/grep/read`: variantes capitalizadas do cliente são canonicalizadas (mapa enviado→original), duplicatas descartadas, membros faltantes entram como decoys "unavailable"; `tool_choice: none` quando o caller não declarou tools; tool_choice forçado é re-apontado ao nome canônico.
- **Credencial**: `Bearer public` sem chave na conexão (keyed lane usa a própria chave).
- **Stream forçado upstream** (o tier responde 403 a corpo non-stream); cliente que pediu JSON recebe o stream **dobrado de volta** em um `chat.completion` único, com nomes das tools restaurados; cliente SSE recebe os chunks reescritos linha a linha.
- Erro no meio do stream vira erro explícito, nunca completion vazio.

## Resultado (backup 2026-10-23, execução real)

| Combo | Antes (PR #161) | Agora |
|---|---|---|
| zn-mimo | fallback até tokenharbor (7,2s) | **OK 1,9s direto por oc/mimo-v2.6-flash-free** |
| zn-free | 403 FreeTierError | **OK 0,7s por oc/glm-5.3-flash-free** |
| zn-glm / zn-gpt / zn-glm-flash | OK | OK |
| zn-deepseek-* / zn-minimax | — | dados do backup (conexão inativa / provider `da` inexistente) |
| zn-grok | — | `gcli/` = openai-responses, exclusão documentada da Fase 3 |

Nota: `zn-free` às vezes responde com content vazio — o modelo gasta o budget de tokens em reasoning (comportamento reportado também pelo cpa-ocfree para modelos nemotron).

## Validação

Testes novos (`opencode_free_test.go`): shape/inversão dos IDs, reaproveitamento de IDs do cliente, canonicalização+decoys+tool_choice, fold do stream com restore de nomes e erro mid-stream. Suíte completa, `vet` e `-race` aprovados no sandbox; imagem reconstruída e a tabela acima reexecutada contra o container real.

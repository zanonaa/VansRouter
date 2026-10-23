# Teste real dos combos com backup do usuário (2026-10-23)

Backup `9router-backup-2026-10-23T00-57-17-221Z.json` importado num container local (`vansrouter:local`, imagem da Fase 6) via mecanismo legado (`db.json` no primeiro boot). Login por `INITIAL_PASSWORD` (senha do backup removida só na cópia de teste). Execução real de chat por combo via `/api/v1/chat/completions` com API key do próprio backup.

## Bugs reais encontrados e corrigidos

1. **Default `stream` invertido** — `stream := body["stream"] != false` fazia o gateway responder SSE a clientes que pediram JSON (padrão OpenAI é não-stream). Corrigido para `== true`.
2. **`baseUrl` do opencode incompleto** — apontava para `https://opencode.ai` (raiz); POST retornava a landing HTML como "sucesso". Corrigido para `https://opencode.ai/zen/v1/chat/completions` (path da free lane na origem).
3. **HTML upstream aceito como sucesso** — upstream 2xx com `text/html` era repassado e **impedia o fallback do combo**. Nova barreira: 502 explícito `Provider X returned HTML...`.
4. **Combo aninhado rejeitado** — entradas sem `/` (ex.: `zn-glm → zn-gpt`) davam 400 `Invalid combo model entry`; a origem as resolve como outro combo. Agora expandidas recursivamente com proteção de ciclo (erro `Circular...` explícito).
5. **`thinking` derrubava resposta Claude/GLM** — bloco `thinking` em resposta não-stream dava 502 `Unsupported Claude content block`. Agora vira `reasoning_content` (fiel a `claude-to-openai`).
6. **`Content-Type` fixo em SSE no sucesso** — resposta JSON vinha com `text/event-stream`; agora preserva o CT do upstream.
7. **Fallback do combo parava em 403** — o Go fazia `break` em 400/401/403; a origem (`accountFallback.js`) só para em erros do pedido (400/422/499) e bloqueio de conteúdo. Política reproduzida (`comboShouldFallback`).

## Resultado final (4/9 combos respondem com texto real)

| Combo | Resultado | Model que respondeu |
|---|---|---|
| zn-glm | ✅ ok | glm/glm-5.3 (via fallback pelos 501 de cx/gcli) |
| zn-gpt | ✅ ok | glm/glm-5.3 (fallback após cx/ e gcli/) |
| zn-glm-flash | ✅ ok | glm/glm-5.3-flash (+reasoning_content) |
| zn-mimo | ✅ ok (7s) | tokenharbor/mimo-v2.5 — **fallback atravessou os dois 403 do OpenCode free** |
| zn-free | ❌ 403 final | todos os 7 models são `oc/*` → FreeTierError do OpenCode (último erro repassado, fiel à origem) |
| zn-grok | ❌ 501 | `gcli/` = grok-cli = formato `openai-responses` — executor especial **excluído por decisão na Fase 3** |
| zn-deepseek-pro | ❌ 400 | conexão deepseek está `isActive: false` no backup |
| zn-deepseek-flash | ❌ 400 | idem |
| zn-minimax | ❌ 400 | `da/` não existe como provider em nenhum catálogo (entrada órfã do backup) |

As 5 falhas restantes não são defeitos do gateway: são dados do backup (conexão desativada, provider inexistente, conta OpenCode free sem sessão de cliente) e a exclusão documentada dos executores `openai-responses` (Fase 3).

## Validação

- Testes novos: `combo_fixes_test.go` (política de fallback, expansão aninhada com ciclo, thinking→reasoning, barreira de HTML).
- Suíte completa, `vet`, `-race` e build aprovados no sandbox; imagem Docker reconstruída e o teste acima reexecutado contra o container.

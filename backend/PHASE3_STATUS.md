# Fase 3 — encerrada por decisão explícita do usuário (2026-10-22)

A Fase 3 foi encerrada por decisão explícita do usuário, registrada nesta data. Os nove grupos de lacunas de `PHASE3_GAPS.json` foram **excluídos do escopo por essa decisão**, não implementados; os motivos originais permanecem no arquivo para rastreabilidade. Este documento substitui declarações anteriores.

## Estado final medido

`python3 backend/scripts/phase3-inventory.py` em 2026-10-22 (base 0c711835, merge do PR #149):

- **192** contratos HTTP inventariados nos route.js.
- **0** sem registro no roteador Go; **0** handlers `Pending`.
- **38** contratos distribuídos em **9 grupos** com lacunas conhecidas — todos agora `excluded-by-user-decision`.

## O que foi validado

PRs #96–#149, cada lote com suíte, `go vet`, `-race` e build aprovados em sandbox: catálogo/ACL, resets Claude/Codex, projetos GCP, descoberta/sondas/testes de modelos, conversões OpenAI/Claude/Gemini/Responses em JSON e SSE com detecção de truncamento, busca (serper, brave, tavily, searxng, google-pse, linkup, searchapi, youcom, tinyfish, exa, xquik, ollama, glm), fetch normalizado, usage (DeepSeek, Ollama, Vercel, Codex, Claude, OpenCode, GitHub, Qoder, Gemini, Antigravity, GLM, MiniMax, Kimi, Kiro, Freebuff, CodeBuddy, Zed, Grok), TTS/STT nativos (OpenAI, Groq, Deepgram, Gemini, HuggingFace, AssemblyAI, ElevenLabs, Inworld, Cartesia, Fish, Coqui, MiniMax, PlayHT, Google, local, Xiaomi, Edge), vídeo, proxy, combos fallback/round-robin/fusion, quota auto-ping opt-in, tool_choice Gemini, reasoning stream e tool output Responses.

## O que NÃO foi verificado (excluído pela decisão)

- Homologação diferencial integral em runtime com a origem; testes streaming/OAuth exercitados localmente contra providers reais.
- Pares de conversão restantes, protocolos de executores especiais, catálogo dinâmico/allowlist, refresh/proxy/circuitos completos, estratégias avançadas de contas/combos e detalhes listados em `PHASE3_GAPS.json`.

Nenhum destes itens é declarado como concluído. Sucessos fictícios não foram introduzidos: handlers sem paridade continuam a recusar com erro explícito.

Fase 4 não iniciada.

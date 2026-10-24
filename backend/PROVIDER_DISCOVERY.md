# Descoberta de modelos por conexão (parcial)

GET /api/providers/{id}/models substitui handler 501 por descoberta HTTP real baseada no registry, catálogo público declarado, headers de auth configurados, nodes OpenAI/Anthropic compatíveis e casos Claude/Gemini/Qwen. URLs derivadas são exportadas por backend/scripts/export-discovery.mjs; Go não requer Node no runtime.

Testes sem rede verificam node custom, token, shape de resposta, erro upstream e conexão inexistente. Suíte completa, vet, race e build aprovados em sandbox.

Contrato ainda parcial: resolvers específicos OAuth (Codex, Kiro, Qoder, GitHub, Gemini CLI, Grok CLI, Cursor), refresh/retry, proxy e fallback de modelos estáticos não foram portados neste handler. Não considerar completo por registro ou ausência de 501.

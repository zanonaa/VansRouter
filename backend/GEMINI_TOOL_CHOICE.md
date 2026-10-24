# Gemini tool_choice

OpenAI auto/none/required e function específica agora viram functionCallingConfig AUTO/NONE/ANY, com allowedFunctionNames quando há nome. Escolhas incompatíveis continuam sendo recusadas.

A origem não traduz tool_choice nesse conversor e usa VALIDATED quando ferramentas existem. Esta mudança preserva a intenção do cliente em vez de descartá-la. Testes cobrem os quatro casos. Suíte, vet, race e build aprovados em sandbox.

Pendências: modos Gemini adicionais, assinatura de pensamento, refresh/proxy e homologação diferencial. Não encerra a Fase 3.

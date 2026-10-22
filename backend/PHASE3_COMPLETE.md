# Fechamento dos recursos avançados da Fase 3

- OAuth genérico usa configuração real para autorização, troca de token e device code. Providers sem URL recebem erro explícito.
- Validação de chaves usa os endpoints de descoberta exportados, não apenas OpenAI, Anthropic e Gemini.
- Tradução OpenAI das etapas 2 e 3 produz corpo, URL e headers reais. Formatos especiais continuam com erro explícito.
- Usage implementa DeepSeek, Ollama, Qwen e iFlow. Providers sem API devolvem a mensagem original de indisponibilidade.
- Settings grava proxy e estratégias de combo. O proxy válido é aplicado imediatamente ao ambiente; a rotação é reiniciada.

Validação: suíte, vet, race e build em sandbox.

Limite restante: tradutores de formatos especiais, refresh/rotação completa de todas as contas e todos os executores de usage ainda não têm paridade integral com o Node. Não há sucesso simulado nesses caminhos.

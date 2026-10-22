# Deduplicação de refresh

TranslatorSend coordena refresh por provider, conta e hash do token antigo. Requisições simultâneas compartilham o resultado; requisições atrasadas reutilizam a rotação durante 30 segundos. Falhas não ficam em cache. Esperas respeitam cancelamento e resultados devolvem cópia do mapa superior.

Testes verificam 12 requisições concorrentes com somente uma chamada upstream, reutilização tardia, isolamento do mapa e cancelamento de quem aguarda. Suíte, vet, race e build aprovados em sandbox.

Coordenação é local à instância TranslatorSend, não entre processos; metadata aninhada não deve ser modificada pelo consumidor. A classificação de erro permanente e demais protocolos continuam pendentes. Não encerra Fase 3.

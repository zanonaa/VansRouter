# Validação de destino público

Fetch valida protocolos, credenciais, nomes internos, endereços literais e todos os resultados DNS. Recusa faixas IPv4 privadas/especiais (incluindo CGNAT, multicast e reservado), IPv6 local e mapeamentos IPv4 privados. Consulta DNS limitada a cinco segundos. Mensagens seguem os erros da barreira original.

Testes usam resolver injetado, sem DNS/rede externa, para respostas privadas/mistas, nomes internos, credenciais, faixas especiais e respostas públicas. Suíte, vet, race e build aprovados em sandbox.

Não implementa pinning de conexão nem revisão de redirects do transporte; chamadas a APIs terceiras recebem a URL validada, mas o destino da extração é controlado pelo serviço externo. Não encerra Fase 3.

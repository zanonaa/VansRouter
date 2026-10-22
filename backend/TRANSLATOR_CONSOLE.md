# Translator console logs

GET, DELETE e stream de /api/translator/console-logs com buffer real, limite de 1000 linhas, entrega inicial e assinantes SSE. PM2 e Docker ficam indisponíveis quando não há runtime externo. Teste cobre append e clear. Suíte, vet, build e race aprovados em sandbox.

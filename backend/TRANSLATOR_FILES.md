# Translator: load e save

GET /api/translator/load e POST /api/translator/save portados. Oito nomes permitidos, diretório DATA_DIR/logs/translator, sem caminhos arbitrários. Load: 400 parâmetro ausente/nome inválido, 404 ausente, 200 success/content. Save exige presença de content (string vazia permitida), cria diretório e escreve UTF-8; erro em tipos não-string retorna 500, não sucesso simulado.

Testes: allowlist/traversal, ausência, conteúdo ausente e roundtrip em diretório temporário. Suíte completa, vet, build e race aprovados em sandbox. Falta de espaço durante a primeira linkedição race resolvida removendo somente binários temporários identificados de lotes anteriores.

Divergências: mensagens de filesystem e JSON do runtime Go não são idênticas às de Node; formato de erro de tipo foi aproximado ao Node. Console logs, stream, send e translate permanecem pendentes, assim como a equivalência integral do bloco.

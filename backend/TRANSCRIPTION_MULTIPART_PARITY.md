# Transcrição multipart

Preserva language, prompt, response_format e temperature como o executor compatível original, nome e bytes do arquivo. Verifica erros de codificação e cópia, remove temporários multipart e limita a chamada a 300 segundos. Repassa Content-Type upstream, permitindo text/plain em vez de anunciar JSON incorretamente.

Teste injetado verifica os campos, conteúdo de áudio, deadline e resposta textual. Suíte, vet, race e build aprovados em sandbox. Demais providers e ACL completa continuam pendentes; não encerra Fase 3.

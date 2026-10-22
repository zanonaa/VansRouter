# Usage OpenCode, GitHub e Qoder

OpenCode Zen/Go consultam seus endpoints, normalizam rolling/weekly/monthly, limitam percentuais e distinguem 401, 403 e entitlement. GitHub usa token OAuth e headers originais, com formatos pago e gratuito e erro explícito sem token. Qoder normaliza quotas, expiração e excedente.

Testes injetados cobrem percentuais textuais, entitlement, snapshots e ausência de token. Suíte, vet, race e build aprovados em sandbox.

Pendências: corpo textual exato de erros GitHub, proxy/refresh e demais adapters. Não encerra a Fase 3.

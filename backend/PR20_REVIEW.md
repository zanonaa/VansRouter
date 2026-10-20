# Revisão e fechamento do PR #20

## Corrigido nesta revisão

- Inicialização do schema no executável, antes de servir HTTP. Snapshot gerado de schema.js, sem Node em runtime.
- Upgrade aditivo e transformações das migrations 002/003/004/007/008, preservando tabelas/colunas desconhecidas. Versão superior à suportada é recusada.
- Backup consistente via VACUUM INTO antes de alterar banco existente. Falha no backup impede alteração; falha no schema reverte a transação. Backups ainda são gerados em cada inicialização com banco existente: retenção/eliminação não é automática neste lote.
- Teste de providers agora parte de banco novo e usa o inicializador real, em vez de schema manual.
- Coerção String e spread para arrays/objetos nos campos de proxy e providerSpecificData, com fixtures geradas pelo Node para comparação direta.
- Preservação de ACL moderna: [] só é convertido a NULL para schemaVersion < 2, como na migration de origem.

## Ainda bloqueia a conclusão solicitada

- Importação JSON legada: não foi portada. Banco novo com arquivos legados e sem marcador é recusado antes de criar schema. Isso evita ignorar dados, mas não é paridade com importação automática da origem.
- Homologação HTTP diferencial completa (Node versus Go) e teste TCP do fluxo completo.
- Validação/conectividade de nós e pools: endpoints ainda não portados; exigem transporte e política de saída reais, sem stubs de sucesso.
- Exceções de bulk: mensagem fixa do Go ainda difere de error.message da origem.
- Casos JS fora do formato comum (objetos como identidade, UTF-16 com surrogate isolado, nomes não escalares, JSON primitivo) ainda exigem comparação e correção. Fixtures atuais não provam paridade universal.
- Descrição do PR e documentos históricos contêm estados antigos; este documento registra o estado desta revisão, sem apagar o histórico.

## Validação

Suíte completa, vet, build e race aprovados em sandbox sem rede. Testes de migration cobrem banco novo, preservação de tabela desconhecida, ACL atual/antiga, backup, rollback e recusa de legado JSON sem mutações. Isso não equivale à conclusão de todas as pendências acima.

Sem merge autorizado por esta revisão. Não avançar para outro lote até concluir o escopo acordado ou renegociá-lo explicitamente.

# Fase 3 — medição em 2026-10-21

Base revisada: merge do PR #59 (f8ef41f1). A medição conta pares verbo/caminho, não arquivos route.js.

## Contagem reproduzível

`python3 backend/scripts/phase3-inventory.py` encontrou **192 contratos HTTP** exportados nos arquivos route.js:

- **81** têm registro literal correspondente no roteador Go (42,2%).
- **111** não têm registro comprovado pela análise estática (57,8%).

Registro NÃO significa implementação completa. O roteador OAuth genérico possui ações/providers ainda com 501/400, enquanto alguns loops de registro (logs) existem mas não são reconhecidos pelo script. Portanto, 81 não pode ser divulgado como número de contratos concluídos; 111 não é contagem exata de rotas ausentes. Aliases/rewrite públicos de next.config também exigem revisão e não estão somados aqui.

### Sem registro comprovado, por grupo

| Grupo | Contratos |
|---|---:|
| v1 | 40 |
| providers | 11 |
| oauth (rotas específicas) | 8 |
| pxpipe | 8 |
| usage | 8 |
| headroom | 7 |
| proxy-pools | 7 |
| translator | 7 |
| models | 5 |
| media-providers | 4 |
| v1beta | 4 |
| version | 2 |
| **Total** | **111** |

## Quanto falta realmente

Mais da metade dos contratos ainda não possui cobertura comprovada e o maior esforço continua aberto: **gateway de IA, tradução de formatos, SSE/streaming, refresh/rotação e ACL aplicados ao tráfego real**. Não é honesto converter a quantidade de PRs ou providers parcialmente portados em percentual de esforço concluído.

Faltam os seguintes blocos antes de fechar Fase 3:

1. Gateway /v1, /v1beta e aliases públicos; transporte, formatos por provider, streaming/cancelamento/backpressure e autenticação de API key.
2. OAuth restante, callbacks/proxies locais, imports e refresh; corrigir diferenças já documentadas (ex. onboarding Antigravity).
3. Providers: testes, descoberta de modelos, GCP, circuit breakers e seleção de contas.
4. Modelos dinâmicos/catalog-sync/availability/test, mídia por provider e execução de áudio/imagens/vídeo etc.
5. Operações remanescentes: translator, pxpipe, headroom, versão/update/restart, quotas e logs/uso em runtime.
6. Homologação integrada com dados populados e regressões, inventário manual sem 501/stubs para contratos reivindicados e documentação final dos desvios aprovados.

React/Vite, remoção SSR/Next, empacotamento final e tag rc pertencem às fases seguintes; não contam como Fase 3 pronta.

## Critério de saída

Todos os contratos no escopo classificados manualmente, implementados ou excluídos por decisão explícita; nenhum sucesso fictício; suite/vet/race/build; testes por boundary e runtime diferencial; streaming e OAuth exercitados localmente; PR/merge da fase sem pendências ocultas.

## Encerramento — 2026-10-22

A Fase 3 foi encerrada por decisão explícita do usuário. Aplicação do critério:

- **Classificação**: 192 contratos inventariados; 0 sem registro; 0 `Pending`; 38 contratos em 9 grupos marcados `excluded-by-user-decision` em `PHASE3_GAPS.json` (não implementados; motivos preservados).
- **Sem sucesso fictício**: lacunas excluídas permanecem documentadas; executores sem paridade recusam com erro explícito.
- **Suite/vet/race/build**: aprovados por lote nos PRs #96–#149 e reexecutados no PR de encerramento.
- **Runtime diferencial, streaming e OAuth locais**: NÃO executados; essa dispensa é parte integrante da decisão explícita do usuário e fica registrada como desvio aprovado, não como verificação.

O PR de encerramento formaliza a decisão e encerra a fase. Fase 4 não iniciada.

# PR #21 — revisão consolidada

Este arquivo substitui o estado de acompanhamento nos comentários antigos. Não declara concluída a migração inteira nem homologa produção.

## Implementado e verificado

- Schema novo/upgrade com backup consistente, rollback e preservação de tabelas desconhecidas. Boot sem mudança não gera backup. Backup SQLite protegido com modo 0600.
- Importação dos quatro JSON legados com backup, transação, contagem de registros, originais intactos, retry após erro e recuperação do marcador sem reimportação após commit. Arquivo modificado entre leitura/backup é recusado.
- Comparações com repositórios JS originais: 8 operações nodes, 22 pools/fitness e 17 connections. Timestamps e UUIDs são normalizados nos testes.
- Contratos de handlers originais: 11 criação nodes, 16 criação/update pools, 12 leitura/delete pools, 11 criação providers, 14 GET/PUT/DELETE providers, 9 bulk e 10 validação nodes. Verificam status/payload com dependências isoladas. São cenários selecionados, não cobertura exaustiva de todas as entradas possíveis.
- NextResponse real (dependência local Next 16.2.9) verifica serialização, status e content-type de todos os sete conjuntos de fixtures. Não usa substituto nessa verificação; ainda não equivale a executar a aplicação Next inteira.
- Smoke TCP do executável Go com importação, guard, login, CRUD, sessão após restart, histórico sem duplicação e backups sem repetição.
- POST provider-nodes/validate: models, chat fallback, Anthropic e embeddings; bloqueio de redes privadas para callers remotos. DNS fixado após validação; redirects remotos recusados.
- POST proxy-pools/{id}/test: proxy HTTP(S) e relays, resultado persistido. Testes com loopback, CONNECT+TLS real e CA fictícia verificada, sem InsecureSkipVerify e sem rede pública. Guard/OPTIONS das novas rotas testados.
- Correção de panic em identidades JSON não comparáveis e recuperação de marcador mesmo quando arquivos originais forem corrompidos após commit.

Validação final: go test ./..., go vet ./..., go test -race ./..., build e smoke TCP aprovados. Testes Go em namespace de rede isolado, módulos/fonte somente leitura, env limpo e limites de recursos. Check NextResponse também executado em sandbox sem rede; heap Node limitado a 256 MiB (reserva virtual de 16 GiB necessária ao Wasm do Node).

## Diferenças intencionais, não bugs escondidos

- JSON legado inválido/ambíguo causa erro em vez de ser ignorado; banco já populado não é sobrescrito automaticamente.
- Backup é integral e obrigatório antes de upgrade (origem usa backup parcial best-effort); não se apagam backups alheios automaticamente.
- Mensagens de rede/erros internos são saneadas em vez de reproduzir mensagens variáveis do Undici e do SQLite nativo.
- SSRF: IP fixado, redirects remotos recusados, IPv6 multicast bloqueado e resposta do validator limitada a 4 MiB. Ver CONNECTIVITY.md.

Essas diferenças precisam constar da aprovação do PR; não há declaração de paridade byte a byte.

## Homologação runtime executada

Next.js 16.2.9 iniciado do mesmo source (cópia descartável) via webpack dev e binário Go lado a lado em namespace sem rede externa. 16 requests comparadas por TCP: guard sem sessão, login/cookie HttpOnly+SameSite, catálogo, criação/listagem nodes, criação/get/update/delete/404 providers, criação/listagem pools, bulk inválido e entrada inválida do validator. Status e payload coincidiram após normalizar UUIDs/timestamps. Content-type JSON verificado; ver scripts/runtime-differential.py. Ambos processos encerrados por PID. Não testa SSR visual, todas as combinações possíveis ou serviços públicos.

O bloco solicitado está implementado e a bateria selecionada passou. Aprovação de merge deve incluir as diferenças intencionais listadas acima; não há promessa de compatibilidade byte a byte para erros internos, retenção de backups ou políticas SSRF. Renderização/frontend, gateway e endpoints fora do CRUD continuam fora deste fechamento.

A remoção de Next.js, React/Vite, demais 136 endpoints e gateway/streaming pertencem às fases futuras; este PR não os implementa.

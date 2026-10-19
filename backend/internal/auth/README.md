# Base de autenticação — Fase 3, lote parcial

Este pacote está ligado ao executável e implementa `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/status`, `GET /api/auth/check`, `POST /api/auth/reset-password`, `GET /api/settings` e `PATCH /api/settings`. **Não conclui a migração da autenticação e não está pronto para corte de produção.** A leitura de settings usa a tabela existente, sem criar tabelas ou migrations. Em banco novo retorna configuração vazia; falhas de consulta/JSON não são tratadas como banco novo.

Implementado:

- Emissão JWT HS256 com claims e validade de 24 horas; grant separado de troca de senha por 10 minutos.
- Leitura de JWT_SECRET ou do arquivo DATA_DIR/jwt-secret existente.
- Criação de segredo aleatório com publicação atômica exclusiva e permissão 0600, evitando segredos diferentes em inicialização concorrente Go/Go.
- Cookies auth_token/password_change com Path=/, HttpOnly, SameSite=Lax, Max-Age e regra Secure equivalente ao helper original.
- Política de senha inicial, placeholders, bcrypt custo 10 e truncamento de 72 bytes compatível com bcryptjs. Contagem de comprimento em unidades UTF-16 para compatibilidade com JavaScript.
- Limiter concorrente em memória: 5 falhas, bloqueios progressivos de 30/120/600/1800 segundos, janela de uma hora e limpeza após sucesso.

## Limites e diferenças explícitas

- Apenas HS256 é aceito, diferentemente da seleção implícita da biblioteca jose original; é o único algoritmo emitido pela aplicação. Essa restrição precisa de revisão antes da integração dos endpoints.
- Arquivo jwt-secret vazio/inacessível gera erro, não regeneração silenciosa. A criação usa hard link atômico: em filesystem sem suporte, retorna erro sem substituir o arquivo.
- Teste cruzado Node → Go aprovado com JWT gerado por jose 6.2.3 e hash gerado por bcryptjs 3.0.3, usando segredo/senha fictícios. Fixture versionada em testdata/node-compatibility.json. O sentido Go → Node ainda não foi verificado.
- O helper de cookie preserva a leitura de X-Forwarded-Proto; antes de usá-lo em produção, a entrada deve ter política de proxy confiável. Este pacote não autoriza confiar em headers de identidade enviados pelo cliente.
- Ainda faltam: escrita/migrations completas, API keys/ACL (validação de API key para LLM remoto), OIDC, troca de segredo JWT e gerenciamento global de memória do limiter. O grant continua limitado à troca de senha pelo guard e pelo handler. Login remoto com senha inicial de compatibilidade emite grant; PATCH /api/settings o consome; demais configurações permanecem bloqueadas. Token CLI implementado com persistência em DATA_DIR/machine-id e DATA_DIR/auth/cli-secret. O grant continua limitado à troca de senha pelo guard e pelo handler: verificar apenas a assinatura não basta para autorização. Login remoto com senha inicial de compatibilidade emite grant e o consumidor PATCH existe agora; as demais configurações permanecem bloqueadas até a migração das respectivas funcionalidades.
- auth/check exige sessão antes do handler, salvo requireLogin=false; mantém a diferença original de exigir JWT_SECRET configurado para confirmar authenticated. Token CLI ainda não é aceito.
- Identidade de origem vem do socket TCP, ignorando headers fornecidos pelo cliente. Forwarding headers impedem classificar como acesso local; proxies ainda não têm allowlist de confiança. O limiter agrupa pelo peer TCP e não pelo XFF, podendo agrupar usuários atrás do mesmo proxy até essa integração.
- Status retorna o fallback público quando a leitura de settings falha. Mensagens internas de erro de login foram saneadas; erros de JSON mantêm status 500, mas não reproduzem mensagens variáveis do parser JavaScript. Não declarar paridade byte a byte.
- Cookies de logout removem os quatro nomes existentes, mas headers Expires/Max-Age ainda precisam de comparação diferencial com Next.js. OPTIONS de autenticação também permanece pendente.
- O grant com apenas currentPassword (sem newPassword), o corpo vazio ou apenas a chave protegida `password` não grava settings gerais neste lote: o original grava `currentPassword`/`password` como setting e incrementa revision; aqui devolve a leitura atual (divergência menor documentada). A troca de senha com sucesso incrementa revision e preserva chaves desconhecidas.
- Tipos não string em `newPassword`/`currentPassword` são recusados como fracos/inválidos; o original pode responder 500 em alguns casos — divergência documentada e não silenciosa.
- Assim como a origem, o limiter só limpa entradas por acesso ou sucesso; gerenciamento global de memória ainda não foi modificado.
- Guard reproduzido em Go: SettingsGuard (PATCH grant ou sessão/token/requireLogin=false), LocalOnlyGuard (operador local autenticado) e Middleware geral para /api/* (deny-by-default) com token CLI (machineId/x-9r-cli-token). Rotas de página (/ , /login, /dashboard, /masuk) permanecem responsabilidade do frontend SPA (Fase 4).
- GET /api/settings mescla os defaults (incluindo REQUIRE_API_KEY/HEADROOM_URL) e omite password/oidcClientSecret; inclui hasPassword/oidcConfigured/enableRequestLogs/enableTranslator.
- PATCH /api/settings recusa mutações além da troca de senha com 501 explícito, em vez de aparentar funcionar enquanto os efeitos colaterais do original não migram.
- Assim como a origem, o limiter só limpa entradas por acesso ou sucesso; gerenciamento global de memória ainda não foi modificado.

## Validação realizada

Testes focados `go test -mod=readonly -p=2 ./internal/auth` aprovados em sandbox Bubblewrap sem rede, ambiente limpo, fonte/toolchain/módulos somente leitura, HOME temporário e escrita restrita ao diretório de validação. Limites: CPU 90 s, memória virtual 4 GiB, processos 64, arquivo 256 MiB, descritores 256 e prazo externo 120 s; paralelismo Go limitado a 2. Somente segredos e identidades fictícios nos testes.

Cobertura: criação/persistência concorrente do segredo, permissões, assinatura inválida, expiração de sessão/grant, algoritmo, cookies, senha e progressão do limiter.

Na atualização deste lote, o cache offline foi preparado seletivamente a partir de arquivos locais já disponíveis, incluindo a extração de golang.org/x/sys v0.29.0. A suíte completa `go test ./...`, `go vet ./...` e o build do executável passaram sob o mesmo isolamento, sem rede e com módulos somente leitura. Testes adicionais cobrem login local/remoto, grant, bloqueio, OIDC exclusivo, status/logout/check e leitura de settings existentes/inválidas. O detector de corrida (`go test -race`) passou neste lote com CGO habilitado. Ainda não foi feita comparação HTTP diferencial completa contra Next.js.

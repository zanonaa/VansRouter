# Base de autenticação — Fase 3, lote parcial

Este pacote **ainda não está ligado ao roteador nem ao executável**. Não adiciona endpoints de login, não libera rotas protegidas e não conclui a migração da autenticação.

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
- A compatibilidade criptográfica com a estrutura JWT foi implementada, mas o teste cruzado com jose/bcryptjs ainda não foi executado.
- O helper de cookie preserva a leitura de X-Forwarded-Proto; antes de usá-lo em produção, a entrada deve ter política de proxy confiável. Este pacote não autoriza confiar em headers de identidade enviados pelo cliente.
- Ainda faltam persistência de settings, identidade/token CLI, API keys/ACL, middleware, login/logout/status/check/reset, troca de senha e OIDC. O grant deve continuar limitado à troca de senha pelo middleware e pelo handler: verificar apenas a assinatura não basta para autorização.
- Assim como a origem, o limiter só limpa entradas por acesso ou sucesso; gerenciamento global de memória ainda não foi modificado.

## Validação realizada

Testes focados `go test -mod=readonly -p=2 ./internal/auth` aprovados em sandbox Bubblewrap sem rede, ambiente limpo, fonte/toolchain/módulos somente leitura, HOME temporário e escrita restrita ao diretório de validação. Limites: CPU 90 s, memória virtual 4 GiB, processos 64, arquivo 256 MiB, descritores 256 e prazo externo 120 s; paralelismo Go limitado a 2. Somente segredos e identidades fictícios nos testes.

Cobertura: criação/persistência concorrente do segredo, permissões, assinatura inválida, expiração de sessão/grant, algoritmo, cookies, senha e progressão do limiter.

A suíte completa foi tentada no mesmo isolamento, mas bloqueada porque golang.org/x/sys v0.29.0 não está extraído no cache disponível e o cache é somente leitura. Não foi feito download para contornar o isolamento. Não declarar teste de corrida, build completo ou paridade diferencial aprovados neste lote.

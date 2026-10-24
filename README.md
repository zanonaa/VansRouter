# VansRouter

Dashboard web e gateway de APIs de IA para gerenciar providers, modelos, conexões, chaves de API, combos, quotas e consumo.

[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## Estado desta branch

Este README descreve a **branch `migration/go-backend`**, com backend Go e frontend React/Vite. A migração ainda não foi incorporada à `main`.

- **Release oficial de referência:** `2.0.0`. Não confundir com os builds de desenvolvimento `2.0.<build>` exibidos por `/api/version`.
- **Imagem Dev do fork:** `ghcr.io/zanonaa/vansrouter:dev-go-arm64`, publicada para **Linux ARM64**. A tag é mutável; atualizações exigem baixar a imagem novamente.
- O CI também valida o build para **AMD64 e ARM64**. Isso não significa que a tag Dev acima contenha as duas arquiteturas nem que um build de migração seja uma release oficial.
- Para AMD64, use o build local descrito abaixo ou uma imagem cuja arquitetura e publicação tenham sido verificadas.

## Executar o Dev com Docker (ARM64)

Para uma instalação nova:

```bash
docker pull ghcr.io/zanonaa/vansrouter:dev-go-arm64
docker run -d \
  --name vansrouter \
  -p 127.0.0.1:20128:20128 \
  -v vansrouter-data:/app/data \
  -e INITIAL_PASSWORD='substitua-por-uma-senha-forte' \
  -e NODE_ENV=production \
  --restart unless-stopped \
  ghcr.io/zanonaa/vansrouter:dev-go-arm64
```

Abra `http://localhost:20128/masuk`. A porta fica acessível apenas no host por padrão; para acesso externo, configure um proxy reverso com HTTPS e autenticação adequada.

**Instalações existentes:** preserve o volume/caminho de dados que já utiliza e faça backup antes de atualizar. Não troque de volume apenas para seguir o exemplo. `INITIAL_PASSWORD` inicializa instalações novas; não redefine automaticamente a senha de um banco existente.

### Build local da imagem

Na raiz desta branch, com Docker e BuildKit:

```bash
docker build -t vansrouter:go-local .
docker run -d \
  --name vansrouter \
  -p 127.0.0.1:20128:20128 \
  -v vansrouter-data:/app/data \
  -e INITIAL_PASSWORD='substitua-por-uma-senha-forte' \
  -e NODE_ENV=production \
  --restart unless-stopped \
  vansrouter:go-local
```

A imagem compila a SPA e o binário Go em estágios separados. Node/npm permanecem no runtime para o PXPIPE, que executa seu pacote via subprocesso; o servidor da aplicação é Go, não Next.js.

> Para usar o Compose desta branch: copie `.env.example` para `.env`, defina `INITIAL_PASSWORD` e revise `VANSROUTER_IMAGE` e os volumes antes de executar `docker compose up -d`. O padrão é a imagem Dev ARM64, publicada apenas no loopback do host. Para uma imagem local, use `VANSROUTER_IMAGE=vansrouter:go-local`. Headroom é opcional: `docker compose --profile headroom up -d`; habilite a integração também no dashboard.

## Estrutura

```text
backend/                  API, gateway, autenticação, SQLite e servidor HTTP Go
frontend/
  src/spa/                Entrada e rotas da SPA
  src/app/                Páginas da interface
  src/shared/             Componentes, hooks e recursos compartilhados
  src/catalogs/           Catálogos de modelos, capacidades e preços
  src/config/             Configurações utilizadas pela interface
  public/                 Assets públicos e traduções
  tests/                  Testes JavaScript
    reference/            Referências legadas usadas somente nos testes
  scripts/                Ferramentas de build e validação
Dockerfile                Build integrado Go + SPA
pnpm-workspace.yaml       Workspace do frontend
```

Os transformadores de requisições antigos em `frontend/tests/reference/` não são importados pela aplicação nem incluídos no bundle. A lógica de gateway e streaming pertence a `backend/`.

## Desenvolvimento local

Requisitos: **Go 1.25+**, **Node.js 22.5+** e **pnpm 9.15.9**.

```bash
pnpm install --frozen-lockfile
pnpm test
pnpm build
```

Para executar Go servindo a SPA compilada, em um shell POSIX:

```bash
cd backend
SPA_DIR="$(pwd)/../frontend/dist" \
  INITIAL_PASSWORD='substitua-por-uma-senha-forte' \
  go run ./cmd/api
```

O servidor atende em `http://localhost:20128`. Fora do Docker, os dados ficam em `~/.9router` por padrão; use `DATA_DIR` para escolher outro caminho.

Para desenvolver com hot reload:

1. Execute o Go em um terminal, como acima.
2. Na raiz do repositório, execute `pnpm dev` em outro terminal.
3. Abra `http://localhost:20129`.

O Vite encaminha `/api` para `http://localhost:20128`; `SPA_API_TARGET` permite alterar esse destino. Os comandos `pnpm` da raiz delegam ao pacote `frontend/`.

### Validação

```bash
pnpm test
pnpm build
node frontend/scripts/check-spa-secrets.cjs
cd backend
go test ./...
go vet ./...
go test -race ./internal/http/handlers/...
go build ./...
```

## Configuração essencial

| Variável | Uso |
| --- | --- |
| `PORT` | Porta HTTP do Go; padrão `20128`. No container, mantenha a porta interna compatível com o mapeamento. |
| `DATA_DIR` | Banco e dados persistentes; `/app/data` na imagem Docker. |
| `SPA_DIR` | Diretório da SPA compilada; `/app/spa` na imagem ou `frontend/dist` no build local. |
| `INITIAL_PASSWORD` | Senha inicial do dashboard em uma instalação nova. Defina uma senha forte. |
| `JWT_SECRET` / `API_KEY_SECRET` | Segredos de autenticação. Quando omitidos, a aplicação gera e persiste os valores sob `DATA_DIR`. Não use placeholders conhecidos em produção. |
| `NODE_ENV` | Use `production` no deploy para habilitar as verificações de configuração de produção. |

As conexões com providers, chaves de API, combos e opções de Token Saver são gerenciadas no dashboard. Proteja o diretório de dados: ele contém credenciais e histórico de uso.

## Usar a API

Cadastre uma conexão de provider e crie uma chave de API no dashboard. Consulte os IDs disponíveis — providers e combos — antes de enviar a chamada:

```bash
export VANSROUTER_API_KEY='sua-chave-do-dashboard'

curl --fail-with-body http://localhost:20128/v1/models \
  -H "Authorization: Bearer $VANSROUTER_API_KEY"
```

Exemplo OpenAI-compatible, substituindo `provider/model` por um ID disponível na sua instalação:

```bash
curl --fail-with-body http://localhost:20128/v1/chat/completions \
  -H "Authorization: Bearer $VANSROUTER_API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "provider/model",
    "messages": [{"role": "user", "content": "Olá!"}],
    "stream": false
  }'
```

Para SSE, use `"stream": true` e `curl -N`. A disponibilidade de outros formatos e serviços depende do executor e das capacidades do provider; constar no catálogo não garante execução implementada.

## Usage e Token Saver

- **Usage:** grava os tokens informados pelo upstream, incluindo os campos de cache/raciocínio suportados, e calcula custos pela tabela de preços/overrides. Os custos são cálculos do gateway, não a fatura do provider.
- Quando o upstream omite usage, há fallback de estimativa identificado por `estimated`. Consumo antigo que não foi gravado não é reconstruído nem inventado.
- **RTK:** ligado por padrão, comprime resultados de ferramentas quando reconhece um formato compatível. Não reduz necessariamente todas as requisições.
- **Headroom, Caveman, Ponytail e PXPIPE:** dependem das opções e dos requisitos de cada integração; não são todos ligados por padrão.
- Para comparar uma requisição sem Token Saver, use `x-9router-token-saver: off`. O header desliga o pipeline inteiro naquela requisição.
- **Tool Call Guard:** remove referências opcionais inventadas por modelos em tool calls no formato chat-completions. Bypass: `x-9router-tool-guard: off` por requisição ou `VANSROUTER_TOOL_GUARD=off` globalmente. Não equivale a validação da existência de recursos no cliente.

## Operação e atualização

```bash
curl --fail-with-body http://localhost:20128/api/health
curl --fail-with-body http://localhost:20128/api/version
docker logs --tail 100 vansrouter
```

Faça backup consistente do SQLite e dos demais dados persistentes antes de atualizar. Para a tag Dev mutável, execute `docker pull` e recrie o container mantendo o mesmo volume e as configurações. Reiniciar o container sozinho não baixa a nova imagem. Não use `docker compose down -v` para atualizar: isso pode apagar volumes.

## Limitações conhecidas da migração

A migração **não tem paridade total comprovada** com o backend JavaScript original.

- Executores ainda não portados retornam **501**: `kiro`, `cursor`, `antigravity`, `grok-web`, `perplexity-web` e `muse-spark-web`.
- A transformação unificada de parâmetros de raciocínio do JS ainda não tem paridade integral validada no Go. As referências de teste não são implementação de runtime.
- O Tool Call Guard atua em saídas chat-completions; não oferece a mesma proteção em saídas nativas Claude/Gemini/Responses.
- A persistência live de detalhes completos em `requestDetails` ainda não está implementada. Não confundir essa aba com o histórico agregado de Usage.
- Limites, créditos e quotas dos upstreams continuam valendo, inclusive em combos com fallback.

## Licença

MIT — consulte [LICENSE](LICENSE).

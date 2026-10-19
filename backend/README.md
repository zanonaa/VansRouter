# Backend Go — Fase 2

Estrutura inicial do backend. Esta fase **não implementa os contratos existentes**, autenticação, migrations ou frontend SPA. O Next.js existente não foi alterado: sua remoção será feita nas fases aprovadas no plano da raiz.

## Decisões

- **Chi v5:** usa `net/http`, facilita `httptest`, cancelamento e streaming com flush, sem impor um modelo próprio de contexto. Não há timeout global de escrita para não limitar futuros streams de IA.
- **SQLite com modernc.org/sqlite:** implementação compilável sem CGO, com suporte a WAL e integração `database/sql`. Usa o mesmo caminho `DATA_DIR/db/data.sqlite`; não cria um banco PostgreSQL nem exige DATABASE_URL.
- `internal/` contém configuração, conexão e roteamento; `cmd/api` contém o executável. Não há `pkg/` ou diretórios vazios sem consumidor.
- Dependências fixadas em go.mod/go.sum. Toolchain mínimo desta entrega: Go 1.23.4.

## Executar

A partir de `backend/`, com Go instalado:

```bash
go mod download
DATA_DIR=/tmp/opencode/vansrouter-fase2 PORT=20228 go run ./cmd/api
```

Em outro terminal:

```bash
curl -i http://localhost:20228/
```

Resposta: HTTP 200, `Content-Type: text/plain; charset=utf-8`, corpo `Hello World` seguido de quebra de linha.

`PORT` mantém o nome usado pelo backend atual e tem padrão **20128**. O exemplo usa **20228** para não colidir com o Next.js durante o desenvolvimento. Nesta fase o servidor faz bind em todas as interfaces e não possui autenticação; use apenas ambiente local/isolado, sem publicação na internet. Não encaminhe tráfego de produção para este servidor mínimo.

As variáveis são lidas do ambiente; arquivos `.env` não são carregados automaticamente. Não coloque segredos em comandos versionados.

## Configuração e cuidados com dados

| Variável | Uso |
|---|---|
| PORT | Porta TCP 1–65535; padrão 20128 |
| DATA_DIR | Raiz persistente; padrão `~/.9router`, ou `%APPDATA%/9router` no Windows |
| SQLITE_SYNCHRONOUS | OFF/NORMAL/FULL/EXTRA; padrão NORMAL |
| NODE_ENV / pm_id | Identificam execução de produção para a proteção de diretórios temporários |
| DATA_DIR_ALLOW_TEMP | `1` permite explicitamente diretórios de teste em produção |

Como no backend atual, a detecção de temporário considera nomes com `smoke`, prefixo `/tmp/9router-data-` e caminhos temporários específicos do macOS. **Diferença intencional:** em produção, esta implementação retorna erro para esses caminhos sem autorização, em vez de escolher silenciosamente outro banco. Erro de permissão também encerra a inicialização; nenhum banco alternativo é aberto. Caminhos Unix explícitos no Windows são rejeitados.

A conexão verifica `PRAGMA quick_check` antes de configurar WAL e falha sem restaurar/substituir arquivos. Aplica foreign_keys, synchronous, busy_timeout=5000, temp_store, mmap_size e cache_size. Uma conexão por processo serializa operações locais; configurações por conexão ficam no DSN para valerem também após reconexão.

**Nenhuma tabela, migration, versão de schema ou importação legada é executada nesta fase.** É possível abrir um banco novo vazio para validar conectividade, mas os repositórios de aplicação só existirão na próxima fase. Não aponte testes para o DATA_DIR real do operador. WAL altera metadados/arquivos auxiliares do banco: use sempre diretório descartável nesta validação. Escrita simultânea Go/Node ainda não foi homologada.

JWT_SECRET, INITIAL_PASSWORD, API_KEY_SECRET e demais variáveis de autenticação serão reutilizadas na implementação dos respectivos serviços. Não são consumidas nem registradas pelo servidor mínimo.

SIGINT/SIGTERM encerram o servidor graciosamente com prazo de 10 s e fecham a conexão SQLite.

## Validação

```bash
go test ./...
go test -race ./...
go vet ./...
go build -o /tmp/opencode/vansrouter-api ./cmd/api
```

Os testes usam diretórios temporários, verificam configuração, pragmas, persistência de um registro, preservação de arquivo inválido e a rota mínima. O banco real não é necessário. A suíte Next.js não foi executada para esta entrega exclusivamente aditiva no backend Go. A migração React/Vite e a remoção das dependências Next.js continuam fora do escopo da Fase 2.

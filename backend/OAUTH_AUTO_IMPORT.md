# Auto-importação Cursor, Kiro e Zed

Rotas GET/HEAD/OPTIONS `/api/oauth/{cursor,kiro,zed}/auto-import` registradas antes do roteador OAuth dinâmico e sujeitas ao middleware de autenticação existente.

- Cursor: candidatos por plataforma, verificação Linux (`which cursor` ou `~/.local/share/applications/cursor.desktop`), consultas readonly a `itemTable` com prioridade de chaves, normalização de JSON string e fallback sqlite3 CLI com timeout de 10 segundos por consulta. Fallback final `{found:false, windowsManual:true, dbPath}`.
- Kiro: prioridade de `kiro-auth-token.json`, primeiro arquivo alternativo válido, resolução de registro IDC, profile.json com normalização apenas da região do ARN CodeWhisperer para us-east-1.
- Zed: settings credentials_url/server_url, secret-tool (Linux), security (macOS), CredRead por PowerShell (Windows), leitura readonly de system_id em kv_store e UUID quando ausente. Comandos executados com timeout; hooks permitem validar sem ler credenciais reais.

Divergências/limitações: o driver Go SQLite substitui better-sqlite3. Zed ainda não tem fallback sqlite3 CLI para kv_store; falha na leitura usa UUID, como o fallback final original. Testes de keyring usam comandos simulados; macOS e Windows não foram exercitados em sistemas reais. Erros de execução do SO podem trazer texto próprio do Go. Estes pontos não constituem prova de equivalência integral multiplataforma.

Validação: suíte completa, go vet, build e suíte completa com -race executados em sandbox bwrap. Testes locais cobrem SQLite Cursor, prioridade/IDC/ARN Kiro e keyring simulado + system_id Zed.

As rotas Kiro import/import-cli-proxy/social-authorize/social-exchange não fazem parte deste sublote.

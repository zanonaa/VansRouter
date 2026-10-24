# Plano de migração do backend Next.js para Go

## 1. Escopo, base e limites da análise

### Atualização de escopo aprovada pelo usuário

O usuário autorizou explicitamente **remover integralmente Next.js e SSR**, adotando **React com Vite como SPA e backend em Go**. Essa decisão substitui a exigência original de frontend absolutamente intacto apenas no que for necessário à remoção do framework: preservar visual, estilos, componentes reutilizáveis e funcionalidades, mas permitir adaptações de roteamento, carregamento de dados, sessão, inicialização e recursos específicos do Next.js.

- Estado final: nenhum processo, dependência ou configuração ativa de Next.js; nenhuma lógica de servidor executada por páginas React.
- SSR identificado neste documento passa a ser um inventário de trabalho obrigatório, não um impedimento à edição das páginas.
- Dados iniciais devem ser obtidos por APIs Go; segredos, credenciais e identidade privilegiada permanecem no servidor.
- Não substituir Next.js por adaptadores Node permanentes para manter SSR. Dependências auxiliares como PXPIPE continuam exigindo decisão própria; não são justificativa para manter Next.js.
- Esta autorização **não autoriza o merge do PR da Fase 1**. Continuar aguardando comando explícito para integrar e iniciar a fase seguinte.
- As seções abaixo que descrevem a preservação absoluta ou a coexistência com SSR registram o plano original e ficam subordinadas a esta atualização e à seção 14.

- Fase 1 — persona: Analista de Sistemas.
- Base: `main`, commit `c310b17f` (a versão declarada em `package.json` é `1.2.0`).
- Integração: `migration/go-backend`; trabalho: `feat/phase-1-mapping`, no worktree `../wt-phase1`.
- Publicação: fork `zanonaa/VansRouter`. O remoto `origin` (`Vanszs/VansRouter`) oferece apenas leitura à conta disponível; o remoto `fork` permite escrita.
- Inventário: **136 arquivos `src/app/api/**/route.js`**. Não há API Routes em `pages/api`. A contagem é de padrões de caminho, não de operações: rotas dinâmicas atendem várias ações, e um arquivo pode exportar vários métodos.
- Esta entrega altera somente este documento. Nenhum componente, página, estilo, configuração, teste ou implementação foi modificado.
- Análise estática: não foi iniciado servidor, aberto banco real, lido arquivo de credenciais do usuário, executado processo de atualização ou chamado provedor de IA. Não há afirmação de paridade comprovada por execução.
- Os contratos abaixo descrevem o código atual, não uma API idealizada. Objetos extensíveis, respostas de provedores e casos dependentes de runtime precisam de amostras sanitizadas e testes diferenciais antes de se tornarem contratos fechados.

### Convenções do inventário

- Caminhos começam por `/api`, salvo aliases públicos explicitados na seção 2.
- `?campo` identifica query string; `{campo}` identifica objeto JSON; `[]` identifica lista; campos opcionais são indicados por `?` quando necessário.
- Status de sucesso padrão: **200**, exceto os explicitamente indicados. Métodos `OPTIONS` explicitamente exportados geralmente respondem **200 com corpo vazio**, não 204; `/api/health` usa 204.
- Erros administrativos normalmente são `{error: string}`; tradutor frequentemente usa `{success:false,error:string}`. Não unificar esses envelopes na migração.
- Além dos erros listados por handler, aplicam-se os bloqueios globais **401/403** da seção 3 e os erros do framework. JSON malformado não tem tratamento uniforme: vários handlers o convertem em **500**, outros em **400** ou usam corpo vazio.
- Métodos não exportados precisam manter o comportamento observado do Next.js, inclusive `HEAD`/`OPTIONS` implícitos e 405. Não presumir que os padrões automáticos do roteador Go sejam idênticos.
- Fonte de cada linha: `src/app` + caminho informado + `/route.js`; parâmetros `[id]`, `[connectionId]`, `[kind]` e `[...path]` mantêm a grafia do diretório de origem.

## 2. Arquitetura existente e superfície pública

O sistema não é um CRUD simples: combina painel, gateway de IA multimodal, tradução de protocolos, OAuth, SQLite, arquivos locais, ferramentas opcionais e rotinas de inicialização.

```text
custom-server.js / runtime-secrets.cjs
  → src/proxy.js → src/dashboardGuard.js
  → src/app/api/**/route.js
      → src/lib/db/repos + src/lib/localDb.js + src/models/index.js
      → src/lib/auth + src/lib/oauth + src/lib/network
      → src/sse/handlers + src/sse/services
          → open-sse/handlers + services + executors + translator + rtk
```

### Aliases e rewrites obrigatórios

Fonte: `next.config.mjs`. Preservar também acesso direto a `/api/*`.

| URL externa | Destino atual |
|---|---|
| `/v1/v1/:path*`, `/v1/v1` | `/api/v1/:path*`, `/api/v1` |
| `/codex/:path*` | `/api/v1/responses` |
| `/responses` | `/api/v1/responses` |
| `/v1beta/:path*`, `/v1beta` | `/api/v1beta/:path*`, `/api/v1beta` |
| `/v1/:path*`, `/v1` | `/api/v1/:path*`, `/api/v1` |

Peculiaridades: `/responses` não aparece na lista de prefixos públicos do guard; `/v1beta` tem rewrite mas não possui `route.js` próprio em `/api/v1beta`; o catálogo de metadados anuncia `/v1/fetch`, enquanto a rota implementada é `/v1/web/fetch`. Não inventar rotas novas para corrigir essas divergências sem decisão explícita. Verificar em teste a ordem entre guard e rewrites.

`custom-server.js` saneia headers de identidade, deriva o IP do socket e adapta upgrades h2c para HTTP/1.1. Rejeita combinações ambíguas de `Content-Length`/`Transfer-Encoding` e limita esse caminho a 64 MiB. `next.config.mjs` desativa compressão para não atrasar SSE e configura limite de proxy padrão de 128 MiB. São limites diferentes do limite de JSON dos handlers.

## 3. Autenticação, autorização e middlewares

### 3.1 Ordem real do guard

Fontes: `src/proxy.js`, `src/dashboardGuard.js`, `src/lib/auth/trustedPeer.js`, `custom-server.js`.

1. Aplicar restrição local quando o caminho começa com um item da lista local.
2. Aplicar proteção obrigatória quando começa com um item da lista de proteção obrigatória.
3. Para prefixos públicos de IA, exigir acesso local reconhecido, token CLI válido ou API key ativa para acesso remoto.
4. Para os demais `/api/*`, liberar allowlist pública; caso contrário exigir token CLI, JWT ou `settings.requireLogin === false`.
5. Exceção limitada: `PATCH /api/settings` pode aceitar cookie `password_change` válido.
6. Para HTML, manter redirects de `/`, `/login`, `/masuk` e proteção de `/dashboard`.

**Allowlist pública por caminho e subcaminho:** `/api/health`, `/api/ready`, `/api/init`, `/api/locale`, `/api/auth/login`, `/api/auth/logout`, `/api/auth/status`, `/api/auth/oidc`, `/api/version`.

**Prefixos IA:** `/v1`, `/v1beta`, `/api/v1`, `/api/v1beta`, `/codex` (caminho exato ou seguido de `/`). Falha remota: 401 com erro textual e `Access-Control-Allow-Origin: *`.

**Proteção obrigatória** (JWT ou CLI, mesmo com login desabilitado): `/api/shutdown`, `/api/settings/database`, `/api/version/shutdown`, `/api/version/update`, `/api/oauth/cursor/auto-import`, `/api/oauth/kiro/auto-import`, `/api/oauth/zed/auto-import`.

**Restrição local:** os três auto-imports anteriores, `/api/auth/reset-password`, `/api/headroom/start`, `/api/headroom/stop`, `/api/headroom/proxy`. CLI válido libera; navegador precisa ser local e autenticado (ou login desabilitado). Falha: 403. A implementação usa `startsWith`, portanto esses controles não são apenas igualdade de rota. `headroom/restart`, `headroom/extras` e PXPIPE não constam nessa lista: registrar a diferença, não ampliá-la silenciosamente em nome da paridade.

### 3.2 Identidade local e confiança no proxy

- Em produção, não basta o header `Host` indicar localhost. É necessário header de IP carimbado e token de peer confiável, emitido pelo servidor customizado; em desenvolvimento existe fallback de host loopback.
- `x-9r-via-proxy` impede classificar a requisição como local. Quando existe `Origin`, ele também deve ser loopback.
- `custom-server.js` remove headers de identidade enviados pelo cliente, emite `x-9r-real-ip` e `x-9r-peer-token`, e só aproveita IP encaminhado quando o peer TCP é loopback. A presença de forwarding headers marca passagem por proxy.
- Go deve derivar identidade do socket e ter política explícita de proxies confiáveis. Não aceitar headers arbitrários como prova de acesso local. Nginx em container não pode fazer todo cliente remoto parecer local.
- `PUBLIC_API_HOSTS` é lido e normalizado, mas no fluxo atual de `canAccessPublicLlmApi` o resultado não concede acesso: a variável local de host confiável não altera o retorno.

### 3.3 Sessão do painel

Fontes: `src/lib/auth/dashboardSession.js`, `password.js`, `loginLimiter.js`.

- JWT HS256, `iat`, `exp` em 24 horas; emissão inclui `authenticated:true` e claims OIDC opcionais.
- Segredo: `JWT_SECRET`, senão arquivo `DATA_DIR/jwt-secret`; geração aleatória persistida com permissão 0600. Preservar os bytes para aceitar sessões existentes.
- Cookie `auth_token`: HttpOnly, SameSite=Lax, Path=/, Max-Age=86400. Secure quando `AUTH_COOKIE_SECURE === "true"` ou `x-forwarded-proto === "https"`.
- Verificação atual de sessão valida JWT/expiração; não exige explicitamente claim `authenticated` nem issuer/audience para a sessão local. Qualquer reforço deve ser deliberado e testado.
- Cookie separado `password_change`: JWT HS256 com `scope:"password-change"`, 10 minutos, mesmos atributos de proteção. Só libera troca de senha via PATCH settings; não é sessão geral.
- Senha armazenada como bcrypt, custo 10. Sem hash, usa política de `INITIAL_PASSWORD`; valores fracos configurados em produção podem impedir login. O valor de compatibilidade conhecido não é reproduzido aqui como credencial de implantação.
- Primeiro login remoto com senha inicial de compatibilidade recebe somente o grant de troca, não sessão completa.
- Limiter por IP em memória: a cada 5 falhas, bloqueios progressivos de 30 s, 120 s, 600 s e 1800 s; janela de uma hora desde a última falha para limpeza. Sucesso limpa estado. 429 inclui `Retry-After`, `retryAfter`, `resetHint`; 401 inclui `remainingBeforeLock`.
- IP do limiter usa peer confiável; `TRUST_PROXY=true` habilita fallback por XFF; sem identidade confiável usa bucket único.

**Divergência importante:** `/api/auth/check` verifica com `JWT_SECRET` obrigatório, sem fallback do arquivo `jwt-secret`, ao contrário do guard e de `/auth/status`. Pode retornar 401 para cookie aceito por outros caminhos. Não declarar compatibilidade automática sem teste específico ou aprovação de correção.

### 3.4 Token CLI, chaves e ACL

Fontes: `src/shared/utils/machineId.js`, `apiKey.js`, `src/lib/db/repos/apiKeysRepo.js`, `src/sse/services/auth.js`, `internalTrust.js`.

- Identidade da máquina: SHA-256 do ID bruto persistido em `DATA_DIR/machine-id` + salt; primeiros 16 caracteres hexadecimais. Salt geral vem de `MACHINE_ID_SALT` ou padrão do código.
- Token CLI: mesmo algoritmo com salt específico e segredo aleatório adicional persistido em `DATA_DIR/auth/cli-secret`; header `x-9r-cli-token`. A checagem interna de IA exige formato hex de 16 caracteres e comparação em tempo constante, falhando fechada.
- Chave nova: `sk-{machineId}-{keyId}-{crc8}`; `keyId` tem 6 caracteres; CRC é prefixo de HMAC-SHA256 com `API_KEY_SECRET`, senão `DATA_DIR/auth/api-key-secret`. Segredos persistidos não devem ser regenerados durante corte.
- A validação de acesso consulta a chave exata na tabela `apiKeys` e exige ativa; não substituir pela simples validação de formato/HMAC.
- Guard extrai Bearer, `x-api-key`, `x-goog-api-key`, depois `?key`. Handlers comuns extraem apenas Bearer e `x-api-key`; Gemini nativo possui extração própria. Uma chave aceita no guard pode não satisfazer o handler.
- Existem duas camadas: acesso remoto exige chave no guard independentemente de `settings.requireApiKey`; os handlers só montam ACL quando essa configuração exige chave, com exceções específicas. Não colapsar as camadas.
- ACL `allowedProviders`, `allowedCombos`, `allowedKinds`: **null = sem restrição; [] = nenhuma permissão; lista preenchida = seleção**. Campo omitido em atualização não limpa permissões.
- Alias e prefixo de node customizado podem satisfazer ACL de provedor. Combos aceitam prefixo `combo/`; catálogo disponível e modelos desabilitados também limitam execução.
- Tipos efetivamente usados incluem `llm`, `embedding`, `image`, `tts`, `stt`, `webSearch`, `webFetch`, `video`. Não reduzir à lista abreviada de comentários.
- Token interno válido ignora ACL nos handlers que o consultam. `handleFetch` não tem o mesmo bypass explícito de `handleSearch` e das outras modalidades; preservar a diferença até decisão.

### 3.5 OIDC

Fontes: `src/lib/auth/oidc.js`, rotas `auth/oidc`.

- Configuração em settings: `authMode` (`password`, `oidc`, `both`), issuer, client ID/secret, scopes, rótulo.
- Discovery, Authorization Code, PKCE S256, state, nonce e JWKS remoto; troca de código via formulário; verificação de issuer/audience do ID token.
- Cookies `oidc_state`, `oidc_nonce`, `oidc_code_verifier`, HttpOnly/Lax/Path=/, 600 s. Callback compara state, exige cookies, limpa fluxo e emite sessão.
- Helper passa `nonce` às opções de `jose.jwtVerify`, mas não há comparação explícita `payload.nonce === nonce` no código local: confirmar comportamento da biblioteca antes de afirmar proteção equivalente.
- Origem pública prioriza `BASE_URL`, depois `NEXT_PUBLIC_BASE_URL`, depois headers encaminhados/host. Callback deve continuar `/api/auth/oidc/callback` na origem pública.
- Start permanece GET por navegação do navegador; rejeita prefetch e metadados de fetch incompatíveis quando presentes. Ausência dos headers não é rejeitada.

## 4. Contratos administrativos

### 4.1 Saúde, sessão e configurações

| Caminho | Métodos e entrada | Resposta e regras/status específicos |
|---|---|---|
| `/api/health` | GET; OPTIONS | GET `{ok:true}`; OPTIONS 204; CORS `*` |
| `/api/ready` | GET | Consulta `SELECT 1`; `{ok:true,database:"ready"}` ou 503 `{ok:false,database:"unavailable"}`; no-store |
| `/api/init` | GET | **Texto** `Initialized`, não JSON; não dispara bootstrap diretamente |
| `/api/locale` | POST `{locale}` | `{success:true,locale}`; valida lista em `src/i18n/config.js`; cookie `locale`, Path=/, um ano; 400 inválido, 500 falha |
| `/api/auth/login` | POST `{password}` | `{success:true,mustChangePassword:boolean}` + cookie apropriado; 401 senha errada, 429 limiter, 403 senha desativada por OIDC, 503 política inicial inválida, 500 exceção; sucesso no-store |
| `/api/auth/logout` | POST | `{success:true}`; apaga sessão e três cookies OIDC; no-store; não apaga explicitamente `password_change` |
| `/api/auth/status` | GET | `{requireLogin,authMode,oidcConfigured,oidcLoginLabel,hasPassword,displayName,loginMethod,oidcName,oidcEmail,oidcLogin}`; falha interna retorna defaults com 200 |
| `/api/auth/check` | GET | `{authenticated:true}` ou 401 `{authenticated:false}`; rota não é pública no guard; diferença de segredo descrita acima |
| `/api/auth/reset-password` | POST `{newPassword}` | `{success:true}`; mínimo 12 caracteres e não-placeholder, bcrypt; 400 JSON inválido/política, 500 falha; restrição local |
| `/api/auth/oidc/start` | GET navegação | Redirect 307 ao IdP; cookies temporários; 403 prefetch/modo/destino inválidos; configuração/falhas redirecionam a `/login?error=...` |
| `/api/auth/oidc/callback` | GET `?code&state` ou `?error` | Redirect 307 ao dashboard no sucesso; sessão OIDC; falhas redirecionam a login com erro; não é JSON |
| `/api/auth/oidc/test` | POST `{issuerUrl?,clientId?,clientSecret?,scopes?}` | Usa settings como fallback; `{ok,discoveryOk,clientSecretTested,clientSecretValid,issuerUrl,clientId,scopes,redirectUri,authorizationEndpoint,tokenEndpoint,jwksUri,message? ou error?}`; segredo inválido pode retornar **200 ok:false**; 400 obrigatórios, 401 autenticação própria, 500 discovery/falha |
| `/api/settings` | GET | Settings sem `password` e `oidcClientSecret`, acrescidos de `oidcConfigured,enableRequestLogs,enableTranslator,hasPassword`; no-store |
| `/api/settings` | PATCH objeto parcial extensível | Retorna settings seguros + `oidcConfigured`, não necessariamente os flags derivados do GET; remove atribuição direta de password; `{newPassword,currentPassword}` troca senha; 400 política/falta de senha, 401 reautenticação, 403 grant com campos proibidos, 500 falha |
| `/api/settings/database` | GET header `x-9r-password`; POST `{password,...exportacao}` | GET exporta objeto lógico do banco, **não arquivo SQLite**; POST importa e retorna `{success:true}`; proteção obrigatória + reautenticação; handler dispensa senha se header CLI está presente, dependendo da validação anterior do guard; 401 senha, GET 500, POST 400 |
| `/api/settings/proxy-test` | POST `{proxyUrl,testUrl?,timeoutMs?}` | Resultado de `testProxyUrl` (`ok,status,...`); valida campos e destino no serviço `src/lib/network/proxyTest.js`; falha de validação/teste com status derivado, exceção 500 `{ok:false,error}` |
| `/api/tags` | GET; OPTIONS | Objeto de catálogo Ollama de `open-sse/config/ollamaModels.js`; CORS `*`, mas rota exige autorização do painel no guard |

PATCH settings também aplica proxy de saída imediatamente, invalida rotação de combos quando estratégia muda e reconfigura agendador de quota quando flags mudam. `oidcClientSecret` vazio não apaga o segredo existente. Settings são extensíveis: não impor um DTO Go que descarte opções desconhecidas já persistidas.

### 4.2 Chaves, combos e modelos

| Caminho | Métodos e entrada | Resposta e regras/status específicos |
|---|---|---|
| `/api/keys` | GET; POST `{name}` | GET `{keys:[ApiKey]}`; POST **201** `{key,name,id,machineId}`; ID de máquina obtido no servidor; 400 nome, 500 falha |
| `/api/keys/[id]` | GET; PUT `{name?,isActive?,allowedProviders?,allowedCombos?,allowedKinds?}`; DELETE | GET/PUT `{key:ApiKey}`; DELETE `{message}`; 404 inexistente; nome vazio ignorado; listas inválidas normalizadas para null; 500 falha |
| `/api/combos` | GET; POST `{name,models?,kind?,context_length?}` | GET `{combos:[Combo]}`; POST **201 Combo direto**, sem wrapper; 400 nome/duplicidade/contexto, 500 falha |
| `/api/combos/[id]` | GET; PUT objeto parcial; DELETE | GET/PUT Combo direto; DELETE `{success:true}`; 404 ausente, 400 validação; invalida rotação e cache de modelos |
| `/api/models` | GET; PUT `{model,alias}` | GET `{models:[{...modelo,fullModel,routedModel,alias,caps}]}` incluindo fetchers dinâmicos e filtrando desabilitados; PUT `{success:true,model,alias}`; 400 faltantes/alias duplicado |
| `/api/models/alias` | GET; PUT `{model,alias}`; DELETE `?alias` | `{aliases:mapa}`; PUT `{success:true,model,alias}`; DELETE `{success:true}`; 400 faltantes; atenção à ordem dos argumentos do setter, diferente da rota `/models` |
| `/api/models/custom` | GET; POST modelo único `{providerAlias,id,type?,name?,caps?,transport?}` ou `{models:[]}`; DELETE `?providerAlias&id&type?` | GET `{models:[]}`; POST `{success:true,added}` ou `{success:true,count}`; DELETE `{success:true}`; type padrão llm; capabilities booleanas por whitelist, transport só STT válido; 400 faltantes, 500 falha |
| `/api/models/disabled` | GET `?providerAlias?`; POST `{providerAlias,ids:[]}`; DELETE `?providerAlias&id?` | GET `{ids:[]}` filtrado ou `{disabled:mapa}`; mutações `{success:true}`; DELETE sem id reabilita todos do provedor; 400 validação |
| `/api/models/availability` | GET; POST `{action:"clearCooldown",provider,model}` | GET `{models:[{provider,model,status,until?,connectionId,connectionName,lastError}],unavailableCount}`; POST `{ok:true}` limpa locks e estado relacionado; 400 ação inválida |
| `/api/models/catalog-sync` | GET; POST | GET estado de sync + `catalog:null` ou `{syncedAt,models,providers,bytes}`; POST `{success:true,result}` ou 503 `{error}`; lê/escreve catálogo local |
| `/api/models/test` | POST `{model,kind?}` | Resultado de `pingModelByKind`, normalmente `{ok,...}`; kind padrão llm; 400 sem modelo; 500 `{ok:false,error}`; pode chamar inferência real |
| `/api/pricing` | GET; PATCH `{[provider]:{[model]:{input?,output?,cached?,reasoning?,cache_creation?}}}`; DELETE `?provider?&model?` | Mapa de preços direto, mesclando defaults e overrides; PATCH só números não negativos e campos permitidos, 400 inválido; DELETE reseta modelo/provedor/tudo; 500 falha |

Entidades:

- `ApiKey`: `id,key,name,machineId,isActive,createdAt,allowedProviders,allowedCombos,allowedKinds`. As rotas administrativas de chaves retornam a chave completa, ao contrário de estatísticas de uso.
- `Combo`: `id,name,kind,models,context_length,createdAt,updatedAt`; nome com letras/números/ponto/hífen/underscore; contexto null ou inteiro positivo até 2.000.000. Models e estratégia de execução devem preservar ordem e formato atual.
- `GET_DEFAULTS` em `pricing/route.js` é uma função exportada, **não um método HTTP nem uma rota `/api/pricing/defaults` implementada**.

### 4.3 Conexões e nodes de provedores

| Caminho | Métodos e entrada | Resposta e regras/status específicos |
|---|---|---|
| `/api/providers` | GET | `{connections:[],aliasMap:{},providers:[{id,alias,aliases,displayName,noAuth,serviceKinds}]}`; remove apiKey/accessToken/refreshToken/idToken de nível superior; enriquece nomes/aliases |
| `/api/providers` | POST `{provider,apiKey,name?,displayName?,priority?,globalPriority?,defaultModel?,testStatus?,providerSpecificData?,connectionProxyEnabled?,connectionProxyUrl?,connectionNoProxy?,proxyPoolId?}` | **201** `{connection}` sem apiKey; normaliza provedor; tipos apikey/cookie, Ollama local dispensa chave; node compatível precisa existir; 400 validação, 404 node, 500 falha |
| `/api/providers/[id]` | GET; PUT campos editáveis da conexão e proxy; DELETE | GET/PUT `{connection}` sem quatro credenciais de topo; DELETE `{message}`; 404 inexistente; PUT permite name/priority/globalPriority/defaultModel/isActive/testStatus/lastError/lastErrorAt/projectId/isProjectIdManual/providerSpecificData; apiKey só atualiza authType apikey; 400 proxy inválido |
| `/api/providers/bulk` | POST `{items:[{provider,apiKey,name,priority?,providerSpecificData?}]}` | **201** `{results}`; 1–500; valida lote antes de persistir, normaliza nomes/chaves; 400 item inválido, 409 provider/nome repetido, 404 node; criação em transação no repositório |
| `/api/providers/client` | GET `?provider=all&accountStatus=all&sort=priority&page=1&pageSize=20` | `{connections,providerOptions,pagination:{page,pageSize,total,totalPages},totals:{eligibleConnections,providerFilteredConnections}}`; pageSize até 500; só elegíveis a quota, whitelist de campos e PSD, mascara nomes longos; pode preencher emails Codex |
| `/api/providers/validate` | POST `{provider,apiKey?,providerSpecificData?}` | `{valid,error?}`; falha da credencial frequentemente **200 valid:false**; 400 entrada/provedor não suportado, 404 node, 500 exceção; probes específicos por provedor e modalidade; noAuth pode dispensar chave |
| `/api/providers/test-batch` | POST `{mode,providerId?}` | `{mode,providerId,results:[{connectionId,connectionName,provider,valid,latencyMs,diagnosis?,...}],testedAt,summary:{total,passed,failed}}`; modos provider/oauth/free/apikey/compatible/all; 400 modo; atualiza saúde das conexões |
| `/api/providers/[id]/test` | POST corpo opcional com overrides do teste | `{valid,error,refreshed}`; 404 conexão, 500 falha; ver `test/testUtils.js` para probes e efeitos persistidos |
| `/api/providers/[id]/test-models` | POST | `{provider,connectionId,results}`; testa catálogo com conexão fixada; 404 conexão, 400 sem modelos, 500 falha |
| `/api/providers/[id]/models` | GET | `{provider,connectionId,models,warning?}`; consulta modelos upstream, com refresh/caches e normalização por provedor; 404 conexão, 400 configuração/provedor, 401 token, status upstream ou 500; não assumir um único payload bruto upstream |
| `/api/providers/[id]/gcp-projects` | GET | `{projects:[]}`; restrito a provedores suportados, refresh de token; 404 conexão, 400 provedor, 401 token, status upstream, 500 falha |
| `/api/providers/[id]/overrides` | GET; PUT `{headers:{nome:valor}}` | GET `{headers,builtinHeaders}`; PUT `{headers}`; **id representa provedor/alias, não necessariamente conexão**; máximo 20 headers, valores até 8192, sem CR/LF, bloqueia auth/cookie/estrutura HTTP; vazio remove override; 400 inválido |
| `/api/providers/kilo/free-models` | GET | `{models,cached,warning?}`; cache e fallback de cache vencido; 502 `{models:[],error}` sem cache |
| `/api/providers/suggested-models` | GET `?url&type` | `{data:[]}` filtrado; 400 faltantes/tipo; guardedFetch com proteção SSRF e resolução fixada, sem redirects, timeout 15 s; 502 remoto/falha |
| `/api/providers/circuit-breakers` | GET | `{statuses}` dos breakers em memória; 500 falha |
| `/api/providers/circuit-breakers/[name]/reset` | POST | `{success:true,message}`; exige autorização própria; 400 nome, 401, 500; reseta estado CLOSED |
| `/api/provider-nodes` | GET; POST `{name,prefix,type?,apiType?,baseUrl?}` | GET `{nodes}`; POST **201** `{node}`; tipos openai-compatible/anthropic-compatible/custom-embedding; apiType chat/responses no primeiro; UUID com prefixo; defaults e normalização de URL por tipo; 400 inválido |
| `/api/provider-nodes/[id]` | PUT `{name,prefix,apiType?,baseUrl,...}`; DELETE | PUT `{node}` e sincronização de dados nas conexões vinculadas; DELETE `{success:true}` remove conexões relacionadas; 404 ausente, 400 inválido, 500 falha |
| `/api/provider-nodes/validate` | POST `{baseUrl,apiKey,type?,modelId?}` | `{valid,error?,method?,dimensions?}`; probes models/chat/embeddings, SSRF e timeout; erro de autenticação upstream pode ser **200 valid:false**; 400 entrada/URL, 500 falha |

Fonte adicional de configuração de provedor: `src/lib/providerNormalization.js`, `src/shared/constants/providers.js`, `open-sse/providers/registry/`, `open-sse/config/providers.js`. Dados específicos como região, endpoint Azure, deployment, accountId e baseUrl não cabem em um único DTO rígido.

**Proxy por conexão:** atualização aceita `proxyPoolId` ou `proxyPoolIds` deduplicados + `proxyRotationStrategy` (none/fill-first/round-robin/random/smart). Valida existência dos pools; desassociação aceita null, vazio e marcador interno. Mescla PSD existente; mudar apenas parte dos campos legados de proxy tem a normalização específica do handler, não um PATCH genérico.

### 4.4 Pools de proxy

Todas possuem autorização adicional de painel em `src/lib/auth/routeAuth.js`, além do guard.

| Caminho | Métodos e entrada | Resposta e regras/status específicos |
|---|---|---|
| `/api/proxy-pools` | GET `?isActive&includeUsage=true`; POST `{name,proxyUrl,noProxy?,isActive?,strictProxy?,type?}` | GET `{proxyPools}` com `boundConnectionCount` opcional; POST **201** `{proxyPool}`; type http/vercel/cloudflare/deno, padrão http; 400 faltantes |
| `/api/proxy-pools/[id]` | GET; PUT campos parciais do pool; DELETE | GET/PUT `{proxyPool}`; DELETE `{success:true}`; 404 ausente; 409 `{error,boundConnectionCount}` se vinculado a conexão; 400 inválido |
| `/api/proxy-pools/[id]/test` | POST | `{ok,status,statusText,error,elapsedMs,testedAt}`; persiste resultado; teste de relay para vercel/cloudflare/deno, teste de proxy para http; 404 ausente, 500 falha |
| `/api/proxy-pools/fitness` | GET | `{pools}` snapshot de indisponibilidade por escopo; 500 falha |
| `/api/proxy-pools/[id]/fitness/clear` | POST `{scope}` | `{ok:true,poolId,scope}`; 400 faltantes, 500 falha |
| `/api/proxy-pools/fitness/clear-all` | POST `{provider?}` | `{ok:true,provider}`; null limpa todos; 500 falha |
| `/api/proxy-pools/import-public` | POST `{source,protocol?,count?,country?,anonymity?,httpsOnly?,maxSpeed?,orderBy?}` | `{source,file,fetched,imported,duplicates,failed,created:[{id,name,proxyUrl}],reason?}`; normaliza limites por fonte e deduplica URL; 400 filtros/fonte, 429 limite remoto, 502 fonte, 500 falha |
| `/api/proxy-pools/vercel-deploy` | POST `{vercelToken,projectName?}` | **201** `{proxyPool,deployUrl}`; cria deployment remoto e registro; 400 token, status remoto ou 500 |
| `/api/proxy-pools/cloudflare-deploy` | POST `{accountId,apiToken,projectName?}` | **201** `{proxyPool,deployUrl}`; publica Worker, habilita subdomínio; 400 dados/subdomínio, status remoto ou 500 |
| `/api/proxy-pools/deno-deploy` | POST `{denoToken,orgDomain,projectName?}` | **201** `{proxyPool,deployUrl}`; cria app e revisão, acompanha deploy e tenta limpeza em falhas; 400 entrada, 409 nome existente, status remoto ou 500 |

Código de relay aparece como texto dentro de handlers de deploy: não confundir seus `status`/exports com contratos do servidor Next.js. Não executar deploy durante testes de contrato; usar servidores simulados.

### 4.5 Uso e observabilidade

| Caminho | Métodos e entrada | Resposta e regras/status específicos |
|---|---|---|
| `/api/usage/history` | GET `?provider&model&startDate&endDate` opcionais | Lista de histórico; 500 falha |
| `/api/usage/stats` | GET `?period=7d` | Estatísticas com totais e mapas byProvider/byModel/byAccount/byApiKey/byEndpoint, last10Minutes, pending, activeRequests, recentRequests, errorProvider; 400 período, 500 falha |
| `/api/usage/chart` | GET `?period=7d` | `[{label,tokens,cost,requests}]`; buckets horários ou diários; 400 período, 500 falha |
| `/api/usage/logs` | GET | Últimos 200 logs derivados de usageHistory; 500 falha |
| `/api/usage/request-logs` | GET | Mesmo serviço de logs, envelope direto; 500 falha |
| `/api/usage/providers` | GET | `{providers}` derivados de detalhes, conexões e nodes; 500 falha |
| `/api/usage/request-details` | GET `?page=1&pageSize=20&provider&model&connectionId&status&startDate&endDate` | `{details,pagination:{page,pageSize,totalItems,totalPages,hasNext,hasPrev}}`; page >=1, pageSize 1–100; substitui request/providerRequest/providerResponse/response existentes por `{redacted:true}` na lista; 400 paginação, 500 falha |
| `/api/usage/request-details/[id]` | GET | `{detail}` individual, com regras de sanitização/truncamento do armazenamento; 404 ausente, 500 falha |
| `/api/usage/stream` | GET SSE | Payload `{activeRequests,recentRequests,errorProvider,pending}` na conexão e eventos update/pending; comentário ping a cada 25 s; remove listeners no cancelamento |
| `/api/usage/[connectionId]` | GET `?force=1` opcional | Quota do provedor, formato variável; ou `{message}` quando indisponível para esse tipo; refresh pode persistir novas credenciais; 404 conexão, 401 refresh, 500 falha |
| `/api/usage/[connectionId]/claude-reset` | POST `{grantId?}` | Resultado de reset do serviço, `ok` no sucesso; apenas Claude OAuth; 404 conexão, 400 tipo, 409 ou 4xx remoto quando não aplicado, 500 falha |
| `/api/usage/[connectionId]/codex-reset-credits` | GET; POST | GET resultado de disponibilidade de créditos; POST resgata crédito e retorna `{code,reset,windows_reset,redeemRequestId?,credit?,message?}`; apenas Codex OAuth/access-token; 400 tipo, 404 conexão, 401 refresh, 409 sem crédito, 4xx remoto/502 resposta inesperada, 500 falha |

Períodos aceitos: today/24h/7d/30d/60d/all. Datas/buckets usam semântica de timezone local do processo e rótulos definidos pelo código; preservar ou aprovar mudança. Estatísticas usam mascaramento de API key, inclusive nas chaves dos mapas: não expor chave bruta ao portar.

### 4.6 Vozes e ferramentas operacionais

| Caminho | Métodos e entrada | Resposta e regras/status específicos |
|---|---|---|
| `/api/media-providers/tts/voices` | GET `?provider=edge-tts&lang?&apiKey?` | `{voices,languages,byLang}` normalizados; provedor por registry, ElevenLabs pode usar chave fornecida; 400 sem fetcher, 502 falha |
| `/api/media-providers/tts/elevenlabs/voices` | GET `?lang?` | `{languages,byLang}` ou `{voices}` filtrado; primeira conexão ativa; inclui free_users_allowed; 400 sem conexão, 502 falha |
| `/api/media-providers/tts/deepgram/voices` | GET `?lang?` | Mesmo agrupamento; modelos TTS são vozes, deduplicação por idioma; 400 sem conexão, 502 falha |
| `/api/media-providers/tts/inworld/voices` | GET `?lang?` | Mesmo agrupamento; 400 sem conexão, 502 falha |
| `/api/media-providers/tts/minimax/voices` | GET `?provider=minimax|minimax-cn&voice_type=all&lang?` | Mesmo agrupamento; separa vozes de sistema/clonadas/geradas/música; verifica erro de negócio mesmo com HTTP 200 upstream; 400 sem conexão, 502 falha |
| `/api/headroom/status` | GET | Resultado de detecção + `{url,managedPid}`; consulta URL configurada; 500 falha |
| `/api/headroom/start` | POST | `{success:true,...resultadoProcesso}`; só gerencia destino loopback; usa porta/configuração de settings; 400 proxy externo/não instalado, 500 falha |
| `/api/headroom/restart` | POST | Mesmo contrato de start; restrição global diferente de start conforme seção 3 |
| `/api/headroom/stop` | POST | Resultado `{stopped,...}`; 409 se não parou, 500 `{error,code}` |
| `/api/headroom/extras` | GET `?log=1` opcional; POST/DELETE `{extras:[]}` | GET `{log}` ou `{available,...statusExtras}`; instalação/desinstalação retorna resultado do processo; 400 NO_PYTHON/NOT_INSTALLED/INVALID_EXTRAS conforme método, 500 falha |
| `/api/headroom/proxy/[...path]` | GET/POST/PUT/PATCH/DELETE/HEAD/OPTIONS, query e corpo encaminhados | Resposta do Headroom, **não só JSON**; remove hop-by-hop, cookie/auth para host não-loopback, preserva status; reescreve fetches específicos do HTML dashboard; redirects manuais; 500 erro |
| `/api/pxpipe/status` | GET | `{installed,installing,version,path,running,loadedAt,uptimeMs,npmAvailable,mode:"library",enabled,autoInstall,minChars,timeoutMs}` |
| `/api/pxpipe/start` | POST | Status PXPIPE; auto-instala quando habilitado; 409 NOT_INSTALLED, 500 `{error,code}` |
| `/api/pxpipe/stop` | POST | `{stopped,...status}`; descarrega módulo em memória, não daemon; 500 falha |
| `/api/pxpipe/restart` | POST | Status após recarregar módulo; 500 `{error,code}` |
| `/api/pxpipe/install` | POST | Informações de instalação + `{health}`; instalação pode levar minutos; 500 `{error,code}` |
| `/api/pxpipe/health` | GET/POST | `{healthy,checks:[{id,label,ok,detail}],error}`; resultado de verificação malsucedida pode ser **200 healthy:false**; exceção 500 |
| `/api/pxpipe/logs` | GET `?limit=100` até 500 | `{installLog,events}` recentes invertidos; 500 falha |
| `/api/pxpipe/stats` | GET `?limit=100` até 500 | `{windows:{all,today,yesterday,last7d,last30d},timeline,recent}`; totais de requests/compressed/bypassed/errors/tokens/tempo/imagens; 500 falha |
| `/api/version` | GET | `{currentVersion,latestVersion,hasUpdate,buildId}`; consulta npm com cache 5 min, timeout 4 s, backoff de falha 30 s, deduplicação de chamada; flags de teste podem suprimir consulta |
| `/api/version/update` | POST | `{success:true,message}`; inicia atualizador e agenda saída; 403 fora de produção; proteção obrigatória |
| `/api/version/shutdown` | POST `{packageName?,mode?:"auto"|"manual"}` | `{success,runtime,mode,autoRestart,autoRestartCommand? ou installCommand?,message}`; encerra em ~1,5 s, pode lançar processo destacado; proteção obrigatória |
| `/api/shutdown` | POST header Authorization Bearer correspondente a `SHUTDOWN_SECRET` | `{success,message}`; 403 produção, 401 sem segredo correspondente; agenda saída em 500 ms; o JWT/CLI exigido pelo guard não substitui a checagem interna. O handler chama `headers().get` sem await: validar compatibilidade com a versão Next atual antes de afirmar que o desligamento funciona |

**Dependência crítica PXPIPE:** atualmente é pacote JavaScript carregado no processo. Um servidor Go puro não pode importá-lo diretamente. Decidir entre portar o transformador, isolar um serviço auxiliar com contrato próprio ou renegociar suporte. Não declarar migração concluída deixando essa funcionalidade desaparecer.

### 4.7 Tradutor e console

| Caminho | Métodos e entrada | Resposta e regras/status específicos |
|---|---|---|
| `/api/translator/translate` | POST `{step,body}` | Etapa 1 `{success:true,result:{provider,model,sourceFormat,targetFormat}}`; etapa 2 `{success:true,result:{body}}`; etapa 3 usa provider/model no body interno e retorna `{success:true,result:{url,headers,body}}`; 400 campos/etapa/conexão, 500 falha |
| `/api/translator/send` | POST `{provider,model,body}` | SSE upstream no sucesso; `{success:false,error,details?}` nas falhas, status upstream preservado; 400 campos/conexão, 500 falha |
| `/api/translator/load` | GET `?file` | `{success:true,content}`; arquivos permitidos por whitelist no handler; 400 faltante/nome, 404 ausente, 500 falha |
| `/api/translator/save` | POST `{file,content}` | `{success:true}`; grava apenas nomes permitidos em DATA_DIR/logs/translator; 400 inválido, 500 falha |
| `/api/translator/console-logs` | GET; DELETE | GET `{success:true,logs,pm2Logs,pm2Info:{available,errorLogFile,outLogFile,errorCount},dockerLogs,dockerInfo:{isDocker,available}}`; DELETE `{success:true}` limpa buffer; 500 `{success:false,error}` |
| `/api/translator/console-logs/stream` | GET SSE | Eventos data JSON `{type:"init",logs}`, `{type:"lines",lines}`, `{type:"clear"}`; ping 25 s; init só quando buffer não vazio; headers no-cache/no-transform e X-Accel-Buffering:no |

Etapa 3 do tradutor retorna headers construídos para upstream, potencialmente sensíveis. A classificação atual exige sessão administrativa; a migração não deve torná-la pública nem incorporar respostas reais em fixtures.

## 5. OAuth dos provedores

OAuth de provedor configura credenciais do gateway; **não é o login OIDC do painel**. Em geral exige autorização administrativa e persiste `providerConnections`.

### 5.1 Rota dinâmica `/api/oauth/[provider]/[action]`

Fonte única: `src/app/api/oauth/[provider]/[action]/route.js`; definições em `src/lib/oauth/providers.js`, `providers/`, `services/` e `utils/server.js`.

| Método/ação | Entrada | Saída e comportamento |
|---|---|---|
| GET authorize | `?redirect_uri` opcional, demais parâmetros como meta | Dados de autorização do provedor, incluindo URL/state/PKCE conforme implementação; redirect padrão local de callback |
| GET device-code | `?start_url&region&auth_method` opcionais para Kiro | Dados de device flow + `codeVerifier`; geração com ou sem PKCE por provedor; 400 fluxo incompatível |
| GET start-proxy | `?app_port&state?&code_verifier?&redirect_uri?` | `{...resultado,serverSide}`; apenas Codex/xAI; inicia listener de callback e registra sessão; 400 inválido |
| GET poll-status | `?state` | `{status:"unknown"}` ou sessão terminal ou `{status}`; apenas Codex/xAI; sessão terminal é consumida/limpa; 400 inválido |
| GET stop-proxy | sem corpo | `{success:true}`; apenas Codex/xAI; efeito colateral em GET |
| POST exchange | `{code,redirectUri,codeVerifier?,state?,meta?}` | `{success:true,connection:{id,provider,email?,displayName?}}`; troca e persiste tokens/expiração; Cline/Kimchi dispensam verifier; código com aparência JWT segue importação access_token sem troca |
| POST poll | `{deviceCode,codeVerifier?,extraData?}` | Sucesso `{success:true,connection:{id,provider}}`; pendente/erro `{success:false,error,errorDescription?,pending,fatal?}` frequentemente **200**; no sucesso persiste conexão e normaliza kimi-coding para kimi |
| POST manual-code | `{code,state}` | `{success:true,connection}`; só xAI, usa sessão registrada e limpa listener; demais 400 |

Ação desconhecida ou campos ausentes: 400; JSON POST inválido: 400; exceções: 500 `{error}`. Preserve metadados de PKCE/máquina dos fluxos Qoder/Kiro/Kimi e diferenças entre a lista sem PKCE de início e a de polling: não há uma configuração genérica única já comprovada para todos.

### 5.2 Rotas estáticas (precedência sobre a rota dinâmica)

| Caminho | Métodos e entrada | Resposta e regras/status específicos |
|---|---|---|
| `/api/oauth/codex/import-token` | POST `{accessToken,name?}` | `{success:true,connection:{id,provider,email,name,workspace,plan}}`; trim, extrai claims para exibição, aceita token opaco, authType access_token; 400 token, 500 falha |
| `/api/oauth/codex/bulk-import` | POST conta única, lista ou `{accounts:[]}` | `{success:number,failed,results:[{index,ok,id? ou error?}]}`; cada conta exige accessToken, ignora campos controlados pelo servidor, preenche identidade/expiração; execução serial para prioridades; 400 envelope/JSON |
| `/api/oauth/grok-cli/bulk-import` | POST mesmos envelopes | `{total,success:number,failed,results}`; aceita camelCase/snake_case de tokens/expiração, preenche email; falhas parciais 200; 400 envelope/JSON |
| `/api/oauth/cursor/import` | GET; POST `{accessToken,machineId}` | GET `{provider,method,instructions,requiredFields}`; POST `{success:true,connection:{id,provider,email}}` após validação upstream; 400 campos, 500 falha |
| `/api/oauth/cursor/auto-import` | GET | `{found:true,accessToken,machineId}` ou `{found:false,error? ou windowsManual:true,dbPath}`; lê SQLite local via driver/CLI e verifica instalação em Linux; restrição local + obrigatória; 500 falha |
| `/api/oauth/gitlab/pat` | POST `{token,baseUrl?}` | `{success:true}`; valida usuário remoto e salva PAT como OAuth sem refresh/expiração; 400 JSON/campo, 401 remoto, 500 falha |
| `/api/oauth/iflow/cookie` | POST `{cookie}` | `{success:true,connection:{id,provider,email,apiKey:mascarada,expireTime}}`; exige BXAuth, consulta e renova chave upstream, persiste cookie; 400 regra/entrada, status upstream, 500 falha |
| `/api/oauth/kiro/api-key` | POST `{apiKey,region?}` | `{success:true,connection:{id,provider,email}}`; valida profiles, authType api_key, horizonte de expiração longo sem refresh; 400 campo, 500 mensagem de falha sanitizada |
| `/api/oauth/kiro/import` | POST `{refreshToken,clientId?,clientSecret?,region?,authMethod?,profileArn?}` | Mesmo sucesso mínimo; usa IDC regional se clientId/secret presentes, senão social; 400 token, 500 falha |
| `/api/oauth/kiro/import-cli-proxy` | POST auth bruto ou `{cliProxyAuth:...}` / `{auth:...}` / `{json:...}` | Mesmo sucesso mínimo; normaliza external_idp; **400** inclusive erro de importação |
| `/api/oauth/kiro/auto-import` | GET | `{found:true,refreshToken,source,clientId,clientSecret,region,authMethod,profileArn}` ou `{found:false,error}`; lê cache SSO e profile do IDE, normaliza região do ARN; restrição local + obrigatória; 500 falha |
| `/api/oauth/kiro/social-authorize` | GET `?provider=google|github` | `{authUrl,deviceCode,userCode,expiresIn,interval,provider}`; converte milissegundos para segundos; 400 provedor, 502 remoto, 500 falha |
| `/api/oauth/kiro/social-exchange` | POST `{deviceCode,provider,targetProvider?}` | `{success:false,pending:true,error}` ou sucesso mínimo; targetProvider só kiro/kiro-*; deduplica identidade e atualiza conexão existente; 400 entrada, erro remoto classificado, 500 falha |
| `/api/oauth/zed/import` | POST `{accessToken,userId,systemId?}` | `{success:true,connection:{id,provider,email,displayName}}`; gera systemId se ausente, valida sessão e organização; 400 campos, 401 token, 500 falha |
| `/api/oauth/zed/auto-import` | GET | `{found:true,userId,accessToken,systemId,credentialsUrl}` ou `{found:false,error,credentialsUrl}`; keychain/keyring/Credential Manager e kv_store locais; restrição local + obrigatória; 500 falha |

**Não executar auto-imports como levantamento:** seus payloads contêm credenciais reais do host. Acesso a esses caminhos deve ser testado com arquivos e keyrings simulados, não com os dados do operador.

## 6. Gateway IA: contratos e comportamento de streaming

### 6.1 Inventário completo das rotas v1/v1beta

Todas as linhas abaixo exportam OPTIONS além do método funcional indicado, inclusive o reexport `/api/v1`. Os headers CORS específicos devem ser comparados por rota, não substituídos por um middleware único com padrões diferentes.

| Caminho | Método e entrada | Resposta/semântica |
|---|---|---|
| `/api/v1` | GET | Reexport da listagem de modelos |
| `/api/v1/models` | GET | `{object:"list",data:[Model]}`; catálogo construído por `src/sse/services/allowedModels.js`, filtra ACL; 401 chave, 500 falha |
| `/api/v1/models/[kind]` | GET | Mesmo envelope filtrado por modalidade; 404 kind inválido; ACL negada pode produzir lista vazia; 401 chave |
| `/api/v1/models/info` | GET `?id&kind?` | `{id,name,kind,owned_by,endpoint,params?,capabilities?,options?,dimensions?,contextWindow?,voicesUrl?,searchTypes?,...}`; 400 id, 404 modelo; lookup de catálogo não é inferência |
| `/api/v1/chat/completions` | POST JSON OpenAI `{model,messages,stream?,tools?,tool_choice?,...}` | JSON Chat Completion ou SSE conforme negociação/formato; preserva campos adicionais e conteúdo multimodal |
| `/api/v1/messages` | POST JSON Anthropic `{model,messages,system?,max_tokens?,stream?,tools?,...}` | JSON/SSE Anthropic; usa pipeline handleChat com detecção de formato |
| `/api/v1/messages/count_tokens` | POST `{messages?,system?,tools?,...}` | `{input_tokens:ceil(caracteres/4)}`; estimativa, não tokenizer real; inclui blocos/ferramentas; 400 JSON inválido |
| `/api/v1/responses` | POST JSON Responses `{model,input,instructions?,stream?,tools?,...}` | JSON/SSE Responses; **stream ausente vira false** antes do handler compartilhado |
| `/api/v1/responses/compact` | POST objeto de conversa | Injeta `_compact:true`, reutiliza handleChat; não assumir o mesmo default de stream da rota responses |
| `/api/v1/api/chat` | POST formato de chat aceito pelo pipeline | Converte resposta para Ollama, inclusive NDJSON quando aplicável; modelName fallback no adaptador não remove validações do handler |
| `/api/v1/embeddings` | POST `{model,input,dimensions?,encoding_format?,...}` | `{object:"list",data:[{object:"embedding",embedding,index}],model,usage,...}` conforme core/adaptador; aceita entrada suportada pelo provedor |
| `/api/v1/images/generations` | POST `{model,prompt,...}`; opções stream/binário conforme handler | JSON de imagens, SSE ou binário conforme negociação; conexão preferida por header; fallback por conta; não reduzir a JSON fixo |
| `/api/v1/audio/speech` | POST `{model,input,language?,style?,...}`; `?response_format=mp3|json` | Áudio binário ou JSON; **formato escolhido por query**, estilo limitado a 1000 caracteres; model roteia provedor/voz; 400 faltantes |
| `/api/v1/audio/transcriptions` | POST **multipart/form-data** com `model,file,...` | JSON/texto conforme core/transporte; 400 multipart/campos; arquivo/boundary não são contrato JSON; transport customizado pode selecionar Gemini Live |
| `/api/v1/audio/voices` | GET `?provider&lang?` | `{object:"list",data:[]}`; formato público diferente do agrupamento administrativo; 400 provedor, status remoto, 502 falha |
| `/api/v1/search` | POST `{provider ou model,query,max_results?,search_type?,country?,language?,time_range?,offset?,domain_filter?,content_options?,provider_options?,exa_options?}` | Resposta de busca normalizada pelo core/provedor; aceita aliases de opções Exa; 400 entrada/provedor, ACL e erros comuns abaixo |
| `/api/v1/web/fetch` | POST `{provider ou model,url,format?,max_characters?}` | Dados de extração do core; valida URL pública contra SSRF antes do acesso; 400 URL/configuração; não existe rota implementada `/api/v1/fetch` |
| `/api/v1/videos/generations` | POST JSON ou multipart/bytes upstream | Cria job assíncrono, resposta upstream + `x-9router-connection-id`; preserva bytes salvo remoção de prefixo de modelo no JSON |
| `/api/v1/videos/edits` | POST mesmo encaminhamento | Ação edits do core de vídeo |
| `/api/v1/videos/extensions` | POST mesmo encaminhamento | Ação extensions do core de vídeo |
| `/api/v1/videos/[id]` | GET header `x-connection-id`, `?provider?` | Poll da conta que criou o job; exige conexão ativa compatível; sem rotação de conta; 400 vínculo ausente/incompatível, 404 indisponível |
| `/api/v1beta/models` | GET | `{models:[{name,displayName,description,inputTokenLimit,outputTokenLimit,supportedGenerationMethods}]}` construído a partir de catálogo; 500 envelope Gemini |
| `/api/v1beta/models/[...path]` | POST caminho `modelo:generateContent` ou `modelo:streamGenerateContent` (inclui prefixos de provedor suportados pelo parser); JSON Gemini `{contents,systemInstruction?,generationConfig?,...}` | Converte Gemini → interno → Gemini; ação determina stream; TTS nativo usa encaminhamento Gemini próprio; 400 JSON/modelo, 401 chave, 404 credencial/configuração, 499 cancelamento, 502/504 rede/timeout, 503 exaustão, status upstream |

### 6.2 Regras compartilhadas e limites de paridade

Fontes: `src/sse/handlers/*.js`, `src/sse/services/{model,auth,allowedModels,tokenRefresh}.js`, `open-sse/handlers/`, `open-sse/services/`, `open-sse/translator/`, `open-sse/executors/`.

1. Ler JSON com limite de bytes quando o handler usa `readBoundedJson`: padrão **8 MiB**, configurável por `VANSROUTER_MAX_BODY_BYTES` ou alias `NINEROUTER_MAX_BODY_BYTES`; rejeita Content-Length excessivo antes da leitura e conta bytes de chunks. 400 JSON inválido, 413 excesso. Nem todas as rotas usam esse helper (multipart/vídeo/count_tokens diferem).
2. Detectar formato e negociação `Accept`; chat pode ativar streaming quando Accept contém text/event-stream e stream está ausente. Não aplicar isso por cima do default específico de Responses.
3. Resolver alias/context marker/modelo/combos; consultar disponibilidade e ACL. Falhas: 400 modelo/formato/credenciais, 401 chave, 403 ACL, 404 modelo não disponível, 429/503 exaustão e status upstream derivados.
4. Selecionar credenciais com exclusões, prioridade, estratégia fill-first ou round-robin aderente; mutex por provedor; suporte a conexão preferida `x-connection-id` e seleção estrita nos caminhos que exigem.
5. Refresh de credenciais e persistência de tokens; política específica por provedor, com proxies/pools e locks por modelo. Não portar apenas o HTTP wrapper.
6. Traduzir requisições e respostas, incluindo ferramentas, thinking, assinaturas, usage, imagem/áudio e formato binário de executores especializados. Ponte OpenAI é intermediária, mas há traduções diretas para evitar perdas.
7. Aplicar compressão/transformadores opcionais (RTK, Headroom, Caveman, Ponytail, PXPIPE) com semântica de falha aberta onde implementada: indisponibilidade da otimização não pode derrubar inferência.
8. Contabilizar requisições pendentes, custo/tokens/cache, detalhes de observabilidade e estado de saúde; cancelar timers/listeners/upstream ao desconectar.
9. Classificar 429 por tipo de quota, manter cooldown persistido em `modelLock_*`, fallback e circuit breaker. Busca usa escopo `websearch:{provider}` para não bloquear chat que compartilha credenciais; erros 400/422 de busca não disparam bloqueio/rotação.
10. Vídeo é cobrado por job: só repete criação em outra conta em 401/403/429; **não repetir automaticamente 5xx** que pode ter criado job. Encaminha `idempotency-key`; não suporta combos; provedor padrão atual xAI.

Não existe um schema JSON fechado único para campos extras do gateway ou respostas de upstream. Estruturas Go devem preservar campos desconhecidos (`json.RawMessage`/mapa) até tradução específica, números e arrays vazios, distinguindo omitido/null/false. Campos de erro e mensagens são parte da compatibilidade; a documentação em português não autoriza traduzir mensagens retornadas pela API.

### 6.3 SSE e outros corpos não JSON

- OpenAI: eventos `data:` e terminador `[DONE]` quando exigido pelo formato; Responses/Anthropic possuem eventos/estruturas próprios.
- Gemini convertido: `candidates`, `usageMetadata`, `modelVersion`, finishReason; descarta `[DONE]` e encerra pelo fechamento do stream.
- Preservar ferramentas fragmentadas, ordem de eventos, usage final, finish reason, cancelamento, backpressure e flush imediato. Testar cortes de chunk em qualquer byte, inclusive UTF-8 e delimitadores SSE.
- SSE de console/uso tem ping de 25 s e não usa o protocolo de término de IA.
- Preservar headers de mídia e status quando encaminhados; remover headers de compressão/tamanho que não correspondam ao corpo já descomprimido, como faz o caminho Gemini nativo.
- Proxy de borda deve desligar buffering para streams. CORS `*` do gateway público não deve ser combinado indiscriminadamente com credenciais de cookies do painel.

## 7. Banco de dados e persistência

### 7.1 Tecnologia e localização

**SQLite**, não PostgreSQL/MySQL. Não há migrations `.sql` independentes: SQL é declarado/gerado em arquivos JavaScript.

Fontes canônicas:

- `src/lib/db/schema.js`: `SCHEMA_VERSION = 8`, tabelas/colunas/índices e gerador SQL.
- `src/lib/db/migrations/`: cadeia 1, 2, 3, 4, 7, 8 (não inventar migrations 5/6).
- `src/lib/db/migrate.js`: cadeia versionada transacional, sincronização aditiva, backup e importação de JSON legado.
- `src/lib/db/driver.js`, `adapters/`: seleção de driver por runtime e recuperação; produção requer driver nativo e armazenamento persistente.
- `src/lib/db/paths.js`: **DATA_DIR/db/data.sqlite** e **DATA_DIR/db/backups**.
- `src/lib/dataDir.js`: default Linux/macOS `~/.9router`, Windows `%APPDATA%/9router`; valida caminhos temporários/permissões e pode usar fallback. Reproduzir a resolução ou exigir configuração explícita documentada, sem abrir um banco vazio acidentalmente.

PRAGMAs: WAL, synchronous via `SQLITE_SYNCHRONOUS` (NORMAL padrão), temp_store MEMORY, mmap_size 30000000, cache_size -64000, foreign_keys ON, busy_timeout 5000. Não compartilhar cópias divergentes do volume entre processos. Durante coexistência, mudanças Go/Node devem respeitar locks, revisão de settings e o mesmo arquivo; testar concorrência antes de habilitar escrita dupla.

### 7.2 Schema lógico/físico atual

Tipos abaixo preservam nomes/capitalização SQL; `data/models/value/tokens/meta/capabilities` marcados como JSON continuam TEXT. As definições exatas de constraints e índices permanecem em `schema.js`; não substituir o banco por um schema deduzido apenas dos payloads.

| Tabela | Colunas/constraints principais | Uso |
|---|---|---|
| `_meta` | key TEXT PK, value TEXT NOT NULL | versões, revisão de settings, contadores e metadados |
| `settings` | id INTEGER PK CHECK id=1, data TEXT NOT NULL JSON | singleton extensível |
| `providerConnections` | id TEXT PK, provider/authType TEXT NOT NULL, name/email TEXT, priority INTEGER, isActive INTEGER DEFAULT 1, data TEXT NOT NULL JSON, createdAt/updatedAt TEXT NOT NULL | credenciais, PSD, locks, estratégia e saúde; índices provider, provider+isActive e priority |
| `providerNodes` | id TEXT PK, type/name TEXT, data TEXT NOT NULL JSON, createdAt/updatedAt TEXT NOT NULL | nodes customizados; índice type |
| `proxyPools` | id TEXT PK, isActive INTEGER DEFAULT 1, testStatus TEXT, data TEXT NOT NULL JSON, createdAt/updatedAt TEXT NOT NULL | pools e relays; índices active/status |
| `proxyPoolFitness` | poolId/scope TEXT NOT NULL, until INTEGER NOT NULL, reason TEXT, createdAt/updatedAt TEXT NOT NULL; PK(poolId,scope) | indisponibilidade por escopo; índices pool/scope/until |
| `apiKeys` | id TEXT PK, key TEXT UNIQUE NOT NULL, name/machineId TEXT, isActive INTEGER DEFAULT 1, createdAt TEXT NOT NULL; **allowedProviders/allowedCombos/allowedKinds TEXT via migration 3** | ACL JSON nullable; índice key |
| `combos` | id TEXT PK, name TEXT UNIQUE NOT NULL, kind TEXT, models TEXT NOT NULL JSON, context_length INTEGER, createdAt/updatedAt TEXT NOT NULL | roteamento ordenado; índice name |
| `kv` | scope/key TEXT NOT NULL, value TEXT NOT NULL JSON; PK(scope,key) | aliases, customModels, disabledModels, pricing e outros escopos; índice scope |
| `usageHistory` | id INTEGER PK AUTOINCREMENT, timestamp TEXT NOT NULL, provider/model/connectionId/apiKey/apiKeyName/endpoint TEXT, promptTokens/completionTokens INTEGER DEFAULT 0, cost REAL DEFAULT 0, status/tokens/meta TEXT | histórico; índices timestamp DESC, provider, model, connectionId |
| `usageDaily` | dateKey TEXT PK, data TEXT NOT NULL JSON | agregados diários |
| `requestDetails` | id TEXT PK, timestamp TEXT NOT NULL, provider/model/connectionId/apiKey/apiKeyName/status TEXT, data TEXT NOT NULL JSON | observabilidade; índices timestamp DESC, provider, model, connectionId |
| `cachedProviderModels` | providerId/modelId TEXT NOT NULL, kind TEXT DEFAULT llm, ownedBy TEXT NOT NULL, capabilities TEXT JSON, updatedAt INTEGER NOT NULL; PK(providerId,modelId) | catálogo dinâmico; índices kind/provider |

**Armadilha de schema:** a declaração atual de `apiKeys` não inclui as três colunas ACL; elas são adicionadas pela migration 3 e usadas pelo repositório. Portar apenas `TABLES` perde dados/compatibilidade.

Migrations:

1. Bootstrap idempotente das tabelas/índices.
2. Converte listas ACL antigas `[]` para NULL somente se colunas já existem; necessária pela mudança histórica de semântica.
3. Adiciona três colunas ACL de forma idempotente.
4. Adiciona atribuição de chave em requestDetails.
7. Adiciona context_length a combos.
8. Cria proxyPoolFitness e índices.

BackupSchemaVersion e schemaVersion são metadados distintos. O fluxo faz backup leve antes de mudança de schema, aplica migrations por transação, sincroniza aditivamente e preserva dados legados. Importação inicial de `db.json`, `usage.json`, `disabledModels.json`, `request-details.json` só para banco novo, com backup, contagem/assertions e marcador `db/.migrated-from-json`; rollback e preservação dos JSON em caso de inconsistência.

### 7.3 Regras dos repositórios

- Configurações: read-merge-write transacional; incrementa `_meta.settings_revision`; cache de 5 s verifica revisão entre processos. Não fazer update cego que perca campos.
- Conexões: prioridade e reordenação por provedor em transação, metadados flexíveis em `data`; importação serial evita corrida de prioridade.
- Booleanos SQLite são inteiros, mas a API devolve booleanos; datas são strings de timestamp e alguns caches usam epoch inteiro.
- JSON null e string `"[]"` têm semântica diferente para ACL; não converter ambos em slice nil no Go sem controle de serialização.
- Observabilidade: sanitiza headers/campos sensíveis, limita tamanho e retenção/batching; o detalhe individual e a lista têm níveis diferentes de redução do payload. Portar `requestDetailsRepo.js`, não apenas SELECT das colunas.
- Uso agrega custo/tokens e registra vínculo de API key/conta/endpoint, com mapas mascarados nas respostas.
- Export/import lógico está em `src/lib/db/index.js` e repositórios; a importação pode substituir conjuntos de registros e precisa de testes de integridade/rollback em cópia descartável.

### 7.4 Persistência fora do SQLite

Preservar ou migrar explicitamente: `jwt-secret`, `machine-id`, `auth/cli-secret`, `auth/api-key-secret`, catálogo sincronizado, logs do tradutor, eventos/instalação PXPIPE, dados de ferramentas e arquivos de configuração. Caminhos locais de IDE/keyring são dependências do host, não do volume de banco. Docker altera acesso a esses recursos e pode tornar auto-import indisponível.

## 8. Variáveis de ambiente e runtime

Inventário mínimo confirmado por `.env.example` e módulos consultados; ler referências de cada executor ao portar o grupo correspondente. Nenhum valor secreto foi coletado.

| Grupo | Variáveis e uso |
|---|---|
| Persistência | DATA_DIR, DATA_DIR_ALLOW_TEMP, SQLITE_SYNCHRONOUS; detecção de produção também considera pm_id |
| Sessão/identidade | JWT_SECRET, INITIAL_PASSWORD, AUTH_COOKIE_SECURE, API_KEY_SECRET, MACHINE_ID_SALT, SHUTDOWN_SECRET |
| Acesso | REQUIRE_API_KEY (default de settings), PUBLIC_API_HOSTS (efeito limitado descrito), TRUST_PROXY, DEBUG_AUTH; VANSROUTER_PEER_TOKEN/NINEROUTER_PEER_TOKEN são internos ao servidor |
| Endereço | PORT (20128 no start atual; desenvolvimento 20127), BASE_URL, NEXT_PUBLIC_BASE_URL |
| Proxy saída | HTTP_PROXY, HTTPS_PROXY, ALL_PROXY, NO_PROXY e configuração persistida aplicada pelo serviço de rede |
| Ferramentas | HEADROOM_URL, HEADROOM_PORT; opções PXPIPE e demais ferramentas em settings/módulos |
| Observabilidade | ENABLE_REQUEST_LOGS, ENABLE_TRANSLATOR, OBSERVABILITY_ENABLED (não presumir que flags do exemplo substituem as opções persistidas) |
| Tamanho JSON | VANSROUTER_MAX_BODY_BYTES, NINEROUTER_MAX_BODY_BYTES; limite de proxy Next separado NINEROUTER_PROXY_CLIENT_MAX_BODY_SIZE |
| Release/build | NODE_ENV, RELEASE_BUILD_ID, VANSROUTER_SKIP_UPDATE_CHECK/VANROUTER_SKIP_UPDATE_CHECK, NEXT_DIST_DIR, NEXT_TRACING_ROOT_MODE, NEXT_BUILD_CPUS |
| Sincronização | CLOUD_URL, NEXT_PUBLIC_CLOUD_URL e settings relacionados |

Na Fase 2 manter nomes existentes; não criar DATABASE_URL de PostgreSQL, não registrar DSNs/segredos em logs. A porta pública existente deve continuar acessível por proxy; a porta interna Go precisa ser definida sem colidir com o Next.js durante coexistência. Usar configuração explícita documentada para o bind interno.

## 9. SSR e conflito com frontend absolutamente intacto

A remoção de API Routes pode ser feita por proxy, mas isso **não remove toda lógica de backend do processo Next.js**.

| Arquivo preservado | Dependência de servidor identificada |
|---|---|
| `src/app/masuk/page.js` | Lê settings, cookies, JWT e configuração OIDC; passa initialAuth ao cliente |
| `src/app/(dashboard)/dashboard/endpoint/page.js` | Calcula machineId no servidor e passa como prop |
| `src/app/(dashboard)/dashboard/media-providers/[kind]/page.js` | Lê conexões, nodes e combos diretamente do banco e passa props iniciais |
| `src/app/layout.js` | Importa inicialização de proxy de saída, bootstrap e captura de console por efeitos colaterais |
| `src/proxy.js` / `src/dashboardGuard.js` | Guard de HTML/API depende de banco, JWT, identidade local e redirects |
| `src/instrumentation.js` | Instala fonte de catálogo, inicia sync, reativa quotas expiradas e agenda verificação horária |
| `src/shared/services/bootstrap.js` e `initializeApp.js` | Inicialização singleton, limpeza de conexões e auto-ping de quota após atraso |

**Bloqueio para a Fase 3/4:** converter esses caminhos em SPA exige alterar páginas/layouts ou introduzir adaptadores de serviço mantendo as importações e contratos existentes. A primeira opção viola a restrição principal; a segunda pode preservar frontend, mas mantém SSR/adaptação Node e precisa de desenho/autorização antes de ser implementada. Não remover SQLite/acesso a sessão do Node enquanto esses consumidores existirem.

Proposta a submeter à decisão: primeiro migrar HTTP por estrangulamento, mantendo frontend e SSR existentes; em seguida decidir se adaptadores backend-only devem consultar Go para os consumidores SSR. Uma SPA completa e frontend absolutamente idêntico não devem ser prometidos simultaneamente sem resolver essa contradição.

## 10. Estrutura Go e ordem de implementação proposta

```text
backend/
  go.mod
  cmd/api/main.go
  internal/
    config/
    database/                 # SQLite, migrations, backup e repositórios
    auth/                     # sessão, OIDC, CLI, ACL e limiter
    http/
      router/
      middleware/
      handlers/
    providers/                # registry e adaptadores
    gateway/                  # tradução, streams, fallback e seleção
    observability/
    operations/               # ferramentas, arquivos e ciclo de vida
  tests/integration/
  Dockerfile
```

Chi é a escolha preliminar: compatível com `net/http`, middlewares explícitos, integração direta com `httptest`, streaming via Flusher e baixo acoplamento. Confirmar após prova de conceito de catch-all Gemini, cancelamento e proxy. Não criar `pkg/` sem consumidor externo concreto.

### Branches/worktrees e dependências

| Fase | Branch / worktree | Entrega e condição |
|---|---|---|
| 1 | feat/phase-1-mapping / ../wt-phase1 | Este plano, commit/push/PR; **aguardar comando do usuário para merge** |
| 2 | feat/phase-2-setup / ../wt-phase2 | módulo, servidor mínimo, configuração/SQLite; partir da integração atualizada após merge aprovado |
| 3A | feat/phase-3-auth / ../wt-phase3-auth | saúde, sessão, OIDC, guard, identidade; depende do SQLite/config |
| 3B | feat/phase-3-crud / ../wt-phase3-crud | settings, keys, combos, catálogo, pricing, conexões/nodes e pools |
| 3C | feat/phase-3-oauth / ../wt-phase3-oauth | OAuth/imports, callbacks e refresh; depende das conexões e identidade |
| 3D | feat/phase-3-gateway / ../wt-phase3-gateway | catálogo público, chat/Responses/Anthropic/Gemini/Ollama, tradução e streams |
| 3E | feat/phase-3-media / ../wt-phase3-media | imagem/embedding/áudio/vídeo/busca/fetch e vozes, depende de ACL e seleção |
| 3F | feat/phase-3-operations / ../wt-phase3-operations | uso/observabilidade, tradutor, ferramentas e atualização; resolver dependência PXPIPE |
| 4 | feat/phase-4-integration / ../wt-phase4-integration | proxy por mesma origem, CORS quando necessário, confiança entre hops; sem editar fetch/React |
| 5 | feat/phase-5-qa / ../wt-phase5-qa | comparação diferencial de contratos, testes e lint, registro de limitações |
| 6 | release/phase-6-rc1 / ../wt-phase6-rc1 | builds, composição preservando infraestrutura existente, roteiro manual, tag no commit integrado aprovado |

Cada grupo tem PR para migration/go-backend. Integração não é diretório de implementação. Não sobrescrever Dockerfile/compose existentes na Fase 6 sem preservar serviços/volumes e investigar alterações prévias. Remover worktree só após merge e confirmação de ausência de trabalho pendente.

## 11. Estratégia de integração sem editar React

1. Manter uma única origem pública e as mesmas URLs. Proxy externo roteia `/api/*` e aliases IA para Go; páginas/assets ficam no Next.js.
2. Durante migração parcial, roteamento por grupo com fallback explícito para Node, não fallback silencioso que esconda endpoint ausente no Go.
3. Preservar query string, corpo multipart/binário, headers, cookies, Location e status. Desligar buffering/compressão de SSE; dimensionar limites e timeouts por modalidade.
4. Não liberar cookies para origem `*`. Se acesso cross-origin for realmente necessário, allowlist da origem do painel com credentials e Vary:Origin; manter contrato público IA separado.
5. Não confiar em peer-token efêmero do Node como protocolo pronto entre serviços; definir autenticação/trust do proxy sem expor token ao navegador e sem classificar proxy Docker como operador local.
6. Evitar dois schedulers e duas gravações independentes de estado. Inicialização em Go e SSR Node precisam de proprietário único de cada tarefa.
7. Só retirar handlers Node quando o grupo estiver coberto, testado e encaminhado corretamente; guardar caminho de retorno por configuração e backup do banco.

## 12. Testes, validação e critérios de aceite

### O que foi validado na Fase 1

- Contagem e leitura estática de rotas, exports HTTP, parâmetros/corpos e caminhos de retorno; análise de guard, autenticação, SQLite/migrations e consumidores SSR.
- Inspeção de comandos de teste/lint/build em package.json e configuração Vitest.
- Nenhuma execução de inferência, reset de quota, atualização, deploy, import/export real ou consulta a credenciais do host.
- Não foi executada a suíte de runtime nesta fase documental. Não há resultado de teste Go, pois o projeto Go ainda não foi criado.

### Suíte existente a reutilizar como referência

Comandos do projeto: `pnpm test` (Vitest com `tests/vitest.config.js`), `pnpm test:server` (node:test para h2c), `pnpm lint`, `pnpm build`. O config Vitest exclui explicitamente `all-endpoints-robust.test.js` e não inclui automaticamente arquivos `.test.cjs`/`.mjs`; esse conjunto não é sinônimo de todos os testes do repositório. Baselines em `tests/__baseline__` não são evidência de execução atual.

Referências particularmente relevantes em `tests/unit/`: dashboard-guard, password-bootstrap, remote-password-bootstrap, password-reset-route, trusted-peer-security, custom-server-peer-headers, internal-trust-exploit, apikey-acl-security, handler-acl-enforcement, request-body-limits, db-migration-chain, db-concurrent, settings-revision, compatible-provider-connections, providers-bulk-route, proxy-pool-fitness, proxy-pools-auth, video-providers, xai-video-handler, responses-stream-conversion, request-details-payload-bound. Tradutores têm golden tests e testes de ida/volta em `tests/translator/`.

### Matriz mínima de paridade por handler

- Sucesso, falta de campo, JSON inválido, conteúdo de tipo errado, recurso ausente, conflito, erro interno e erro upstream.
- Comparar status, envelope, nomes de campos, tipos, null/omitido/array vazio, mensagens, headers, cookies/expiração e redirects. Normalizar somente valores não determinísticos declarados (UUID/tempo/tokens aleatórios), não diferenças estruturais.
- Sessões Node aceitas por Go e vice-versa; bcrypt compatível; mesma assinatura de identidade/chaves; rotação de segredo não involuntária.
- Autenticação remota/local/proxy, CLI inválido, requireLogin false, requireApiKey true/false, ACL null/[]/seleção, origin e headers de identidade forjados.
- Banco descartável recém-criado e cópias sanitizadas das versões históricas; migrations idempotentes, rollback, backups, importação legada e contagem de registros; concorrência de prioridades/rotação/configuração.
- Streams fragmentados, UTF-8 dividido, tools/thinking/usage, EOF precoce, timeout, cancelamento antes/depois dos headers e ausência de buffering.
- Arquivos multipart e áudio/imagem binários; preservar boundary no vídeo; não duplicar jobs em erro ambíguo; polling preso à conta original.
- Hosts privados/metadados, DNS/redirect, políticas SSRF e proxy de saída; usar alvos simulados controlados, não infraestrutura do operador.
- Testes de integração simulam chamadas do frontend sem modificar seus arquivos; Go: `go test ./...`, teste de corrida onde suportado e golangci-lint; frontend: lint em modo sem correção automática.

### Portões de aprovação

- Nenhum diff de frontend. Revisar lista de arquivos de cada PR antes de commit e merge.
- Nenhum contrato variável considerado fechado sem fixture sanitizada ou teste do adaptador.
- Nenhum endpoint operacional testado contra produção.
- Todas as 136 rotas inventariadas possuem destino/implementação e verificação; nenhum alias é perdido.
- SSR/SPA e PXPIPE resolvidos explicitamente antes de declarar backend 100% Go.
- Release usa dados descartáveis ou cópia autorizada, volumes persistentes, instruções de retorno e validação manual de login/OIDC, CRUD, inferência/streaming, mídia, quota, observabilidade e operação atrás do proxy.

## 13. Decisões e pendências antes das próximas fases

1. Aprovar merge do PR da Fase 1; até lá, manter worktree e branch para revisão.
2. Resolver a incompatibilidade entre SPA completa e páginas/layouts intactos, incluindo a página de mídia com leitura direta do banco.
3. Definir tratamento da biblioteca JS PXPIPE e dos executores/protocolos especializados; não substituí-los por stubs de sucesso.
4. Aprovar se divergências existentes serão preservadas ou corrigidas em PRs separados: segredo de auth/check, uso real de PUBLIC_API_HOSTS, extração de chave entre camadas, nonce OIDC, aliases anunciados não implementados e diferenças de restrição local.
5. Definir ambiente de teste e credenciais de sandbox para OIDC/provedores quando necessário. Até lá, simular dependências externas.
6. Fixar porta interna Go e topologia de proxy/volume; não mudar URLs nem frontend para resolver colisão.
7. Fechar schemas de payloads dinâmicos na implementação de cada adaptador por leitura aprofundada e testes diferenciais; esta análise estática não garante comportamento atual de serviços externos.

## 14. Plano revisado: React SPA sem Next.js nem SSR

Esta seção substitui as propostas anteriores de manter SSR ou frontend sem qualquer alteração. O inventário dos contratos e do banco permanece válido.

### Distribuição de responsabilidades

- **Go:** APIs, sessão/cookies, OIDC, autorização, identidade local, SQLite, catálogo, tarefas de inicialização, observabilidade e regras de negócio.
- **React/Vite:** renderização no navegador, navegação SPA e consumo das APIs. Nenhuma importação de banco, filesystem, segredo JWT ou módulos exclusivos de servidor no bundle.
- **Proxy reverso/servidor estático:** mesma origem pública, encaminhamento de APIs e aliases, entrega dos assets e fallback para index.html apenas em rotas da SPA. Caminhos de API inexistentes devem retornar erro da API, nunca HTML do frontend.

### Fases e worktrees revisados

1. **Fase 1 atual:** atualizar este plano e o PR existente; aguardar merge autorizado.
2. **Fase 2:** servidor Go e infraestrutura de persistência/configuração, como previsto.
3. **Fase 3:** grupos backend já descritos, incluindo migração das funções anteriormente usadas por SSR e das tarefas disparadas por layout/instrumentation.
4. **Fase 4A — Especialista React:** branch `feat/phase-4-react-spa`, worktree `../wt-phase4-react-spa`; configurar Vite e roteamento SPA, migrar consumidores SSR e recursos específicos do framework.
5. **Fase 4B — DevOps:** branch `feat/phase-4-integration`, worktree `../wt-phase4-integration`; integrar SPA/Go por mesma origem, configurar proxy, assets, fallback, CORS e limites de streaming. Concluir remoção de dependências e entradas de execução Next.js, depois de verificar que não há consumidores restantes.
6. **Fase 5:** acrescentar regressão visual e navegação direta/reload de rotas às verificações de contratos, testes Go e lint frontend. Não exigir compatibilidade de execução dos testes que dependem diretamente de APIs internas do Next.js; portar os testes necessários e documentar substituições.
7. **Fase 6:** build Go e **build Vite**, não build Next.js; composição com backend e frontend estático/proxy, preservando serviços auxiliares necessários. Tag RC somente no estado integrado e aprovado.

Cada fase/subfase usa branch temporária e worktree próprio, com PR para `migration/go-backend`. A branch de integração continua sem implementação direta.

### Trabalho obrigatório no frontend

- Substituir o App Router e recursos `next/link`, `next/navigation`, `next/image`, `next/font` e equivalentes conforme inventário de usos; escolher e justificar o roteador SPA na fase de implementação.
- Preservar URLs de navegação e comportamento dos parâmetros dinâmicos. Manter redirects de login, callback e dashboard sem depender de middleware Next.js; autenticação real continua obrigatoriamente nas APIs Go.
- Trocar a leitura SSR de `/masuk` por consulta à API de status de autenticação, com estado inicial de carregamento e tratamento de falha.
- Trocar o cálculo SSR de machineId e as leituras diretas de conexões/nodes/combos por endpoints Go com autorização e payload mínimo. Reutilizar APIs existentes quando suficientes; novos contratos exigem documentação e testes.
- Remover do layout os efeitos colaterais de servidor e transferir inicialização/schedulers para Go com proprietário único.
- Auditar variáveis `NEXT_PUBLIC_*`: Vite não as expõe automaticamente. Definir mapeamento explícito ou reduzir configuração a URLs relativas, sem tornar variáveis secretas públicas.
- Preservar Tailwind, fontes, ícones, assets, tema, internacionalização, callbacks OAuth, SSE e uploads; substituir recursos do framework sem redesenhar a interface.
- Atualizar scripts, dependências, lockfile, configuração ESLint, entradas HTML/JS e Docker/CI afetados. Investigar arquivos existentes antes de remover qualquer configuração.

### Critérios de aceite revisados

- Não há SSR, runtime Next.js, comandos next nem dependência next na aplicação final; nenhum recurso necessário depende de `/_next`.
- O build do frontend é estático e reproduzível com Vite; deep links e reload funcionam no proxy configurado.
- Nenhum código de banco/credenciais/segredos aparece no bundle do navegador.
- Visual e funcionalidades existentes preservados, com diferenças inevitáveis de carregamento documentadas e verificadas.
- APIs mantêm os contratos mapeados; alterações intencionais ficam explícitas em testes e documentação.
- Alterações de frontend são permitidas apenas no escopo necessário à remoção de Next.js/SSR; não realizar reformulação visual ou refatoração não relacionada.

**Estado da entrega:** plano revisado para revisão, não implementação nem autorização de merge. A Fase 2 permanece bloqueada até comando explícito após a revisão da Fase 1.

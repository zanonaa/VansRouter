# PXPIPE (Token Saver / library mode)

Porta de `src/lib/pxpipe/{install,loader,events,service}.js` e das rotas `src/app/api/pxpipe/{health,install,status,start,stop,restart,logs,stats}/route.js` em `backend/internal/http/handlers/pxpipe*.go`.

- Contratos preservados: `{installed, installing, version, path, running, loadedAt, uptimeMs, npmAvailable, mode:"library"}` no status (+ `enabled/autoInstall/minChars/timeoutMs` das settings, chaves ausentes omitidas como `undefined`), health com checks `installed/module/transform` e textos de erro exatos (`pxpipe not installed`, `Cannot load module: …`, `Self-test failed: …`), install `{…info, health}` 500 `{error, code}` (`NPM_NOT_FOUND`), start 409 `{code:"NOT_INSTALLED"}` + auto-install, stop `{stopped, …status}`, restart recarrega, logs `{installLog(200 linhas), events}` com `limit` cap 500, stats `{windows(all/today/yesterday/last7d/last30d), timeline(30d UTC), recent}` com `savedPct` toFixed(2) e `avgCompressionMs` arredondado.
- Install real: `npm install pxpipe-proxy@latest --no-audit --no-fund --omit=dev` em `DATA_DIR/pxpipe`, PATH estendido, log anexado, timeout 5 min, `package.json` host criado.
- Divergências registradas:
  - A origem importa o módulo `transformAnthropicMessages` in-process (library mode); em Go o mesmo módulo é verificado/executado via subprocesso `node -e` (export check no load; self-test com ping sintético), com cache in-process de `version/loadedAt` para start/stop/restart. A ausência de `node` no PATH estendido faz o load falhar com a mensagem de export faltante (a origem sempre roda sob node).
  - A serialização do install usa mutex instead de promessa compartilhada: chamadas concorrentes aguardam a corrida em andamento e podem reexecutar npm em seguida (no-op quando o pacote já está instalado).
  - O pipeline de transformação de requests (appendPxpipeEvent/transform no gateway) entra no lote do gateway `/v1`.
- Nada de stub de sucesso: testes cobrem NPM_NOT_FOUND, NOT_INSTALLED (409/500), export faltante, falha de self-test, shape das rotas e agregação de stats com arquivo real em temp dir.

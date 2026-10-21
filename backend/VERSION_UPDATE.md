# POST /api/version/update no Go

Espelha `src/app/api/version/update` + `src/lib/appUpdater.js` + `UPDATER_CONFIG`.

- Gate idêntico: fora de `NODE_ENV=production` responde 403 com a mensagem original; em produção mata processos irmãos (ps: 9router/next-server,/bin/app,tray; Windows: node.exe/tray com taskkill), espera 1,5 s quando matou algo, dispara o updater destacado (`node updater.js` com env `UPDATER_*` compia para `DATA_DIR/runtime/updater`, relaunch via npx com `--skip-update`) e agenda `os.Exit(0)` após 500 ms como o `setTimeout(process.exit)` original.
- `/api/version/update` está em ALWAYS_PROTECTED do dashboardGuard: sem autenticação (cookie ou token CLI) responde 401 antes do gate. Registrado no router com POST e OPTIONS.
- Divergência documentada: o caminho Windows usa os mesmos parâmetros do PS original; o updater permanece um script Node (empacotamento atual) — o Go apenas o resolve/copia/executa. Testes em produção injetam KillProcesses/SpawnUpdater; os defaults reais não são executados no CI.

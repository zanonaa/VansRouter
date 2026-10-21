# Headroom (status/start/stop/restart/extras) no Go

Porta de `src/lib/headroom/{detect,process}.js` e de `src/app/api/headroom/*/route.js`.

- Detecção idêntica: `which/where headroom` com PATH estendido (EXTRA_BINS), python >= 3.10 com `pip show headroom-ai` (fallback por versão), `pip list --format=json` para markers (`tree-sitter*`/`torch`/`huggingface-hub`), probe `/health` nos 1,5 s, `DEFAULT_HEADROOM_URL` respeitando `HEADROOM_URL`.
- Processo real: PID/LOG/INSTALL_LOG em `DATA_DIR/headroom`, spawn destacado com log em append, startup de 8 s falhando em saída precoce, stop com SIGTERM + SIGKILL aos 2 s, restart aguarda até 3 s e reinicia; extras via `pip install headroom-ai[proxy,...]` / `pip uninstall -y -markers` com whitelist code/ml, log truncado, status 400/500 mapeados por rota como na origem (INSTALL_FAILED/UNINSTALL_FAILED → 500 na extras; NOT_INSTALLED → 400 no start).
- A origem sobrescreve `extras` do install/uninstall com o estado de markers (spread); mantido.
- Guarda local-only (`/api/headroom/*`) via LocalOnlyGuard; settings lidas do banco (`headroomUrl`, `headroomCodeAware===true`, `headroomKompress!==false`). Testes usam hooks (FindBinary/FindPython/Spawn/Status) sem rede e sem tocar pip real; nada de stub de sucesso.

// Shim do módulo "os" para o bundle do navegador (Fase 4A).
// open-sse/providers/shared.js usa platform()/arch() apenas para fingerprints
// de executores server-side; nenhum caminho client invoca essas funções.
// Devolvemos valores neutros em vez de falhar na importação.
export function platform() {
  return "browser";
}
export function arch() {
  return "";
}
export const EOL = "\n";

const nodeOsShim = { platform, arch, EOL };
export default nodeOsShim;

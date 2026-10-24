import { defineConfig, transformWithOxc } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";

const backendTarget = process.env.SPA_API_TARGET || "http://localhost:20128";
const srcRoot = fileURLToPath(new URL("./src", import.meta.url));

// O App Router permite JSX em arquivos .js; o pipeline Vite/oxc decide a
// linguagem pela extensão. Este plugin pré-processa todo .js de src com o
// transform oficial do Vite forçando o parser JSX (runtime automatic),
// devolvendo código e sourcemap para o restante da cadeia.
function jsxInJs() {
  return {
    name: "vansrouter:jsx-in-js",
    enforce: "pre",
    async transform(code, id) {
      const file = id.split("?")[0];
      if (!file.endsWith(".js") || !file.startsWith(srcRoot + "/")) return;
      const result = await transformWithOxc(code, file + ".jsx", {
        jsx: { runtime: "automatic" },
      });
      return { code: result.code, map: result.map ?? undefined };
    },
  };
}

// Fase 4A — fundação Vite da SPA.
// Aliases de compatibilidade apontam módulos next/* para implementações próprias
// baseadas em react-router; nenhum código do pacote next entra no bundle do navegador.
export default defineConfig({
  plugins: [jsxInJs(), react()],
  resolve: {
    alias: {
      "@": srcRoot,
      "open-sse": fileURLToPath(new URL("./open-sse", import.meta.url)),
      "open-sse/": fileURLToPath(new URL("./open-sse/", import.meta.url)),
      "next/link": fileURLToPath(new URL("./src/compat/link.jsx", import.meta.url)),
      "next/navigation": fileURLToPath(new URL("./src/compat/navigation.js", import.meta.url)),
      "next/dynamic": fileURLToPath(new URL("./src/compat/dynamic.js", import.meta.url)),
      "next/image": fileURLToPath(new URL("./src/compat/image.jsx", import.meta.url)),
      // open-sse/providers/shared.js importa "os" para fingerprints server-side;
      // shim neutro evita módulo Node no bundle (nenhum uso client).
      os: fileURLToPath(new URL("./src/compat/node_os.js", import.meta.url)),
      "node:os": fileURLToPath(new URL("./src/compat/node_os.js", import.meta.url)),
    },
  },
  server: {
    proxy: {
      // Nota (4A): respostas SSE ficam presas no proxy do dev server do Vite
      // (buffering do middleware; flush em proxyRes não resolveu). O smoke
      // valida SSE direto na origem Go; a mesma-origem final com streaming é
      // resolvida na Fase 4B (proxy/servidor estático definitivo).
      "/api": { target: backendTarget, changeOrigin: true, ws: true },
    },
  },
  build: {
    outDir: "dist",
  },
  // Módulos compartilhados open-sse leem process.env em nível de módulo
  // (ex.: config/grokCli.js). No navegador essas leituras devem ser vazias.
  // Exceção deliberada: apenas as duas variáveis públicas do app, sem expor
  // segredos (auditoria 4A: nenhum componente client as lê hoje; defines
  // mantêm override de dev/build disponível quando passarem a ler).
  define: {
    "process.env": {},
    "process.env.NEXT_PUBLIC_BASE_URL": JSON.stringify(process.env.NEXT_PUBLIC_BASE_URL ?? ""),
    "process.env.NEXT_PUBLIC_CLOUD_URL": JSON.stringify(process.env.NEXT_PUBLIC_CLOUD_URL ?? ""),
  },
});

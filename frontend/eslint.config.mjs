import { defineConfig, globalIgnores } from "eslint/config";
import js from "@eslint/js";
import reactHooks from "eslint-plugin-react-hooks";

// Globals de runtime Node 22 disponíveis nos módulos server (open-sse/lib):
// fetch/Web APIs nativas além dos clássicos de timer/buffer.
const nodeGlobals = {
  console: "readonly",
  process: "readonly",
  Buffer: "readonly",
  setTimeout: "readonly",
  clearTimeout: "readonly",
  setInterval: "readonly",
  clearInterval: "readonly",
  setImmediate: "readonly",
  clearImmediate: "readonly",
  queueMicrotask: "readonly",
  structuredClone: "readonly",
  fetch: "readonly",
  Request: "readonly",
  Response: "readonly",
  Headers: "readonly",
  AbortController: "readonly",
  AbortSignal: "readonly",
  URL: "readonly",
  URLSearchParams: "readonly",
  TextEncoder: "readonly",
  TextDecoder: "readonly",
  atob: "readonly",
  btoa: "readonly",
  performance: "readonly",
  crypto: "readonly",
  FormData: "readonly",
  Blob: "readonly",
  File: "readonly",
  ReadableStream: "readonly",
  WritableStream: "readonly",
  TransformStream: "readonly",
  ByteLengthQueuingStrategy: "readonly",
  CountQueuingStrategy: "readonly",
  DOMException: "readonly",
  global: "readonly",
  require: "readonly",
  WebSocket: "readonly",
  EventSource: "readonly",
  FileReader: "readonly",
  CustomEvent: "readonly",
  ResizeObserver: "readonly",
  IntersectionObserver: "readonly",
  MutationObserver: "readonly",
  matchMedia: "readonly",
  navigator: "readonly",
  location: "readonly",
  window: "readonly",
  document: "readonly",
  localStorage: "readonly",
  sessionStorage: "readonly",
  history: "readonly",
  alert: "readonly",
  confirm: "readonly",
  prompt: "readonly",
};

// Fase 4B: sem eslint-config-next/Next. Base @eslint/js recomendada com
// ajustes que preservam o baseline do repo (0 erros) e o plugin react-hooks,
// exigido verde pelo regression guard em tests/unit. Regras novas do
// recommended que não existiam no baseline ficam em warning para limpeza
// incremental; no-undef permanece error por escopo com globals corretos.
const eslintConfig = defineConfig([
  js.configs.recommended,
  {
    plugins: { "react-hooks": reactHooks },
    // O App Router/SPA usa JSX também em arquivos .js (comportamento herdado
    // do eslint-config-next); habilita o parser JSX para js e jsx.
    languageOptions: {
      parserOptions: { ecmaFeatures: { jsx: true }, sourceType: "module", ecmaVersion: "latest" },
    },
    rules: {
      "no-unused-vars": "warn",
      "no-empty": "warn",
      "no-useless-assignment": "warn",
      "preserve-caught-error": "warn",
      "no-dupe-keys": "warn",
      // Regras do recommended ausentes no baseline Next: warning para limpeza
      // incremental sem bloquear o gate de 0 erros.
      "no-useless-escape": "warn",
      "no-dupe-class-members": "warn",
      "no-control-regex": "warn",
      "no-case-declarations": "warn",
      "no-regex-spaces": "warn",
      "no-unreachable": "warn",
      "no-constant-condition": "warn",
      // Sem resolver de módulos do Next: imports next/* são cobertos pelos
      // aliases do Vite (src/compat) e não devem falhar no lint.
      "no-undef": "off",
    },
  },
  {
    files: ["**/*.jsx"],
    languageOptions: { parserOptions: { ecmaFeatures: { jsx: true } } },
  },
  // Server-side modules (open-sse engine, lib) são Node ESM puro sem JSX —
  // no-undef como error para capturar ReferenceError de runtime.
  {
    files: ["src/catalogs/**/*.js", "src/config/**/*.js", "src/utils/modelMetadata/**/*.js", "src/lib/**/*.js", "src/sse/**/*.js"],
    languageOptions: { globals: nodeGlobals },
    rules: {
      "no-undef": "error",
    },
  },
  // Componentes do dashboard: no-undef error para capturar crashes de runtime
  // como handlers renomeados (GitHub Issue #1) e props undefined.
  {
    files: ["src/app/(dashboard)/**/*.js"],
    languageOptions: { globals: nodeGlobals },
    rules: {
      "no-undef": "error",
    },
  },
  globalIgnores([
    ".next/**",
    ".next-cli-build/**",
    "out/**",
    "build/**",
    "dist/**",
    "backend/**",
  ]),
]);

export default eslintConfig;

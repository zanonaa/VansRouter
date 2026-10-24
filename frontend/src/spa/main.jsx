// Fase 4A — entrada da SPA em Vite.
// Os efeitos colaterais de servidor do app/layout.js (initOutboundProxy, bootstrap,
// initConsoleLogCapture) não são importados aqui: inicialização/schedulers já
// pertencem ao processo Go desde a Fase 3.
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import "@/app/globals.css";
import "./fonts.css";
import App from "./App.jsx";

createRoot(document.getElementById("root")).render(
  <StrictMode>
    <App />
  </StrictMode>
);

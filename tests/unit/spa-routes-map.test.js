// Fase 5 — contrato do mapa de rotas da SPA.
// Garante que (1) todo import lazy de rota aponta para um arquivo existente e
// (2) toda página page.js restante em src/app possui rota correspondente na
// SPA — evita perda silenciosa de páginas na migração App Router → react-router.
import { describe, expect, it } from "vitest";
import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");
const routesFile = readFileSync(path.join(root, "src/spa/routes.jsx"), "utf8");

function listPages(dir, acc = []) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) listPages(full, acc);
    else if (entry.name === "page.js") acc.push(full);
  }
  return acc;
}

// Caminho do arquivo para o segmento de rota esperado no App Router:
// src/app/(dashboard)/dashboard/basic-chat/page.js -> dashboard/basic-chat
function appRouteSegments(pagePath) {
  const relative = path.relative(path.join(root, "src/app"), pagePath).split(path.sep);
  relative.pop(); // page.js
  return relative.filter((segment) => !/^\(.*\)$/.test(segment)).join("/");
}

describe("mapa de rotas da SPA", () => {
  const pages = listPages(path.join(root, "src/app"));

  it("toda página do app tem rota registrada na SPA", () => {
    const missing = [];
    for (const page of pages) {
      const segments = appRouteSegments(page);
      // Rotas dinâmicas [kind]/[id] viram :kind/:id na SPA; o teste compara
      // pela forma registrada (segmento estático vizinho).
      const expected = segments
        .replace(/\[kind\]/g, ":kind")
        .replace(/\[id\]/g, ":id")
        .replace(/\[\.\.\.path\]/g, "*");
      if (expected === "") continue; // "/" é o index
      if (!routesFile.includes(`"${expected.replace(/\/:kind|\/:id/g, (m) => m)}"`)) {
        // Fallback: rota dinâmica registrada por prefixo estático do pai
        const staticPart = expected.split("/:")[0].replace(/\/\*$/, "");
        if (staticPart && !routesFile.includes(staticPart)) missing.push(expected);
      }
    }
    expect(missing, `rotas sem registro: ${missing.join(", ")}`).toEqual([]);
  });

  it("todo import lazy de página aponta para arquivo existente", () => {
    const lazyImports = [...routesFile.matchAll(/lazy\(\(\) => import\("([^"]+)"\)\)/g)].map((m) => m[1]);
    expect(lazyImports.length).toBeGreaterThan(10);
    const broken = lazyImports.filter((specifier) => {
      const resolved = specifier.replace(/^@/, "src");
      return !existsSync(path.join(root, resolved));
    });
    expect(broken, `imports quebrados: ${broken.join(", ")}`).toEqual([]);
  });

  it("/dashboard/settings/pricing fica fora do DashboardLayout, como no App Router", () => {
    const pricingIndex = routesFile.indexOf('"dashboard/settings/pricing"');
    const dashboardIndex = routesFile.indexOf('path: "dashboard"');
    expect(pricingIndex).toBeGreaterThan(-1);
    expect(dashboardIndex).toBeGreaterThan(-1);
    expect(pricingIndex).toBeLessThan(dashboardIndex);
  });

  it("spa/ tem apenas os arquivos da fundação (sem páginas duplicadas)", () => {
    const files = readdirSync(path.join(root, "src/spa"));
    const expected = new Set([
      "App.jsx", "authGuard.js", "AuthGuard.jsx", "EndpointRoute.jsx",
      "fonts.css", "main.jsx", "MasukRoute.jsx", "MediaKindRoute.jsx",
      "RootError.jsx", "routes.jsx",
    ]);
    const unexpected = files.filter((f) => !expected.has(f));
    expect(unexpected).toEqual([]);
    expect(statSync(path.join(root, "src/spa/routes.jsx")).isFile()).toBe(true);
  });
});

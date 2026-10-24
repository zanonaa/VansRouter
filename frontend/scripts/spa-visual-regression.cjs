// Fase 5 — regressão de navegação/visual da SPA na mesma origem (Go + dist).
// Para cada rota concreta: navegação direta (deep link), verificação de que a
// página renderizou sem cair no error boundary, reload e re-verificação, e
// screenshot como artefato local (não commitado).
// Uso: node scripts/spa-visual-regression.cjs [--shots DIR]
const { chromium } = require("playwright-core");
const fs = require("node:fs");
const path = require("node:path");

const PORT = process.env.SPA_PORT || "20128";
const BASE = process.env.SAME_ORIGIN === "0" ? "http://localhost:20129" : `http://localhost:${PORT}`;
const PASSWORD = "Smoke-Test-2026!Strong";
const shotsDir = process.argv.includes("--shots")
  ? path.resolve(process.argv[process.argv.indexOf("--shots") + 1])
  : path.join("/tmp/opencode", "spa-visual", new Date().toISOString().replace(/[:.]/g, "-"));

// Rotas da SPA com exemplos concretos para parâmetros dinâmicos.
const ROUTES = [
  { name: "root", url: "/" },            // redireciona para /masuk ou /dashboard
  { name: "landing", url: "/landing" },
  { name: "login", url: "/login" },      // redireciona para /masuk
  { name: "masuk", url: "/masuk" },      // autenticado redireciona para /dashboard
  { name: "dashboard", url: "/dashboard" },
  { name: "basic-chat", url: "/dashboard/basic-chat" },
  { name: "combos", url: "/dashboard/combos" },
  { name: "console-log", url: "/dashboard/console-log" },
  { name: "endpoint", url: "/dashboard/endpoint" },
  { name: "media-web", url: "/dashboard/media-providers/web" },
  { name: "media-kind", url: "/dashboard/media-providers/tts" },
  { name: "media-kind-item", url: "/dashboard/media-providers/tts/sample-id" },
  { name: "media-combo", url: "/dashboard/media-providers/combo/sample-id" },
  { name: "callback", url: "/callback?code=x&state=y", allowProcessing: true },
  { name: "profile", url: "/dashboard/profile" },
  { name: "providers", url: "/dashboard/providers" },
  { name: "providers-new", url: "/dashboard/providers/new" },
  { name: "proxy-fitness", url: "/dashboard/proxy-fitness" },
  { name: "proxy-pools", url: "/dashboard/proxy-pools" },
  { name: "pxpipe", url: "/dashboard/pxpipe" },
  { name: "quota", url: "/dashboard/quota" },
  { name: "token-saver", url: "/dashboard/token-saver" },
  { name: "translator", url: "/dashboard/translator" },
  { name: "usage", url: "/dashboard/usage" },
  { name: "pricing", url: "/dashboard/settings/pricing" },
];

// Erros de renderização que caracterizam o error boundary da SPA.
const ERROR_MARKERS = ["VansAI could not load", "This page could not be found", "Dashboard error"];

function fail(step, message) {
  console.error(`FAIL ${step}: ${message}`);
  process.exit(1);
}

(async () => {
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox"],
  });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const apiFailures = [];
  page.on("response", (res) => {
    if (res.url().startsWith(`${BASE}/api/`) && res.status() >= 500) {
      apiFailures.push(`${res.status()} ${new URL(res.url()).pathname}`);
    }
  });

  // Login uma vez; as demais rotas seguem autenticadas (mesma origem/cookie).
  await page.goto(`${BASE}/masuk`, { waitUntil: "load", timeout: 60000 });
  await page.waitForSelector("input[type='password']", { timeout: 20000 });
  await page.fill("input[type='password']", PASSWORD);
  await page.click("button[type='submit']");
  await page.waitForURL("**/dashboard**", { timeout: 30000 });

  fs.mkdirSync(shotsDir, { recursive: true });
  const results = [];
  for (const route of ROUTES) {
    await page.goto(`${BASE}${route.url}`, { waitUntil: "load", timeout: 60000 });
    await page.waitForTimeout(route.allowProcessing ? 4000 : 1800); // lazy + fetches iniciais
    let body = await page.textContent("body");
    if (route.name === "root" || route.name === "login" || route.name === "masuk") {
      // Rotas de redirecionamento: o destino final precisa estar renderizado.
      const final = page.url().replace(BASE, "");
      results.push({ route: route.name, final });
      continue;
    }
    // Rotas com id inexistente (sample-id) devem renderizar o 404 da SPA.
    const isMissingEntity = route.url.includes("sample-id");
    if (isMissingEntity) {
      if (!/404|could not be found/i.test(body)) {
        fail(route.name, "entidade inexistente deveria renderizar 404");
      }
    } else {
      for (const marker of ERROR_MARKERS) {
        if (body.includes(marker)) fail(route.name, `renderizou error boundary (${marker}) após navegação direta`);
      }
      // Regressão Dev 2026-10-23: layout renderizava, mas o miolo da página não
      // (DashboardLayout sem Outlet / página lazy minúscula). Exigir conteúdo.
      // Pricing fica fora do DashboardLayout (sem <main>), então olha o body.
      if (route.url === "/dashboard/settings/pricing") {
        if (!/Pricing/i.test(body)) fail(route.name, "página de pricing não montou");
      } else if (route.url.startsWith("/dashboard")) {
        const content = await page.evaluate(() => {
          const area = document.querySelector("main div.overflow-y-auto");
          return area ? { html: area.innerHTML.length, text: (area.innerText || "").trim() } : null;
        });
        if (!content || content.html < 400 || content.text.length < 10) {
          fail(route.name, "conteúdo da página vazio (layout/página não montou)");
        }
      }
    }
    const shot = path.join(shotsDir, `${route.name}.png`);
    await page.screenshot({ path: shot, fullPage: false });

    // Reload da rota (deep link real) e re-verificação.
    await page.reload({ waitUntil: "load", timeout: 60000 });
    await page.waitForTimeout(1800);
    body = await page.textContent("body");
    if (isMissingEntity) {
      if (!/404|could not be found/i.test(body)) fail(route.name, "404 após reload");
    } else {
      for (const marker of ERROR_MARKERS) {
        if (body.includes(marker)) fail(route.name, `error boundary (${marker}) após reload`);
      }
    }
    await page.screenshot({ path: shot.replace(".png", "-reload.png"), fullPage: false });
    results.push({ route: route.name, url: page.url().replace(BASE, ""), shot });
  }

  if (apiFailures.length > 0) {
    console.error("falhas de API 5xx durante a varredura:");
    for (const entry of apiFailures.slice(0, 10)) console.error(`  ${entry}`);
    fail("api", `${apiFailures.length} respostas 5xx`);
  }

  console.log(`screenshots: ${shotsDir}`);
  for (const r of results) console.log(`OK ${r.route} -> ${r.final || r.url}`);
  await browser.close();
  console.log("VISUAL_OK");
})().catch((e) => fail("inesperado", e.message));

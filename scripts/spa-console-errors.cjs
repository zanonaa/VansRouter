// Varredura de erros JS no console do navegador em todas as rotas da SPA
// (Fase pós-RC): coleta console.error e pageerror por rota com o build atual.
const { chromium } = require("playwright-core");

const BASE = process.env.SPA_PORT ? `http://localhost:${process.env.SPA_PORT}` : "http://localhost:20141";
const PASSWORD = "Combo-Test-2026!";

const ROUTES = [
  "/dashboard", "/dashboard/basic-chat", "/dashboard/combos", "/dashboard/console-log",
  "/dashboard/endpoint", "/dashboard/media-providers/web", "/dashboard/media-providers/tts",
  "/dashboard/profile", "/dashboard/providers", "/dashboard/providers/new",
  "/dashboard/proxy-fitness", "/dashboard/proxy-pools", "/dashboard/pxpipe",
  "/dashboard/quota", "/dashboard/token-saver", "/dashboard/translator",
  "/dashboard/usage", "/dashboard/settings/pricing", "/landing", "/masuk",
];

function fail(msg) { console.error("FAIL " + msg); process.exit(1); }

(async () => {
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox"],
  });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const errors = [];
  page.on("console", (m) => {
    if (m.type() === "error") errors.push({ kind: "console.error", text: m.text().slice(0, 300) });
  });
  page.on("pageerror", (e) => errors.push({ kind: "pageerror", text: (e.message || String(e)).slice(0, 300) }));
  page.on("requestfailed", (r) => {
    const url = r.url();
    if (url.startsWith(BASE)) errors.push({ kind: "requestfailed", text: `${r.failure()?.errorText} ${url.slice(BASE.length).slice(0, 120)}` });
  });

  await page.goto(`${BASE}/masuk`, { waitUntil: "load", timeout: 60000 });
  await page.waitForSelector("input[type='password']", { timeout: 20000 });
  await page.fill("input[type='password']", PASSWORD);
  await page.click("button[type='submit']");
  await page.waitForURL("**/dashboard**", { timeout: 30000 });

  const perRoute = [];
  for (const route of ROUTES) {
    const before = errors.length;
    await page.goto(`${BASE}${route}`, { waitUntil: "load", timeout: 60000 });
    await page.waitForTimeout(2200);
    perRoute.push({ route, newErrors: errors.length - before });
  }

  // Exceções conhecidas e não-blocking (recusadas explicitamente):
  const IGNORE = [
    "favicon",                        // ausência de favicon em /landing
    "net::ERR_FAILED",                // 401/403 de API durante carregamento
    "Failed to load resource",        // idem: status HTTP de API, não erro JS
    "net::ERR_CONNECTION_REFUSED",
    // Cancelamento de fetch/chunk pela navegação (a varredura troca de rota
    // enquanto o guard e os dynamic() carregam): o navegador aborta o pedido,
    // não é defeito de código. Sem isso todo SPA com lazy-loading reportaria
    // falso positivo.
    "net::ERR_ABORTED",
  ];
  const blocking = errors.filter((e) => !IGNORE.some((p) => e.text.includes(p)));
  for (const e of blocking) console.log(`JS ${e.kind}: ${e.text}`);
  console.log("---");
  for (const r of perRoute) console.log(`${r.newErrors ? String(r.newErrors).padStart(3) : "  0"}  ${r.route}`);
  console.log(`total erros JS (fora ignorados): ${blocking.length}`);
  await browser.close();
  if (blocking.length > 0) fail(`${blocking.length} erros JS no console`);
  console.log("JS_CLEAN");
})().catch((e) => fail(e.message));

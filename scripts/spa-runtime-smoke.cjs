// Smoke de runtime da Fase 4A: SPA Vite (20129, proxy /api→Go 20128) no
// navegador real. Valida guard, login com senha inicial, dashboard e deep link.
// Executado uma única vez contra dados descartáveis; não é teste de regressão.
// Nota: waitUntil "load" (networkidle nunca ocorre com o WebSocket do HMR).
const { chromium } = require("playwright-core");

const BASE = "http://localhost:20129";
const PASSWORD = "Smoke-Test-2026!Strong";

function fail(step, message) {
  console.error(`FAIL ${step}: ${message}`);
  process.exit(1);
}

(async () => {
  const browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || "/usr/bin/chromium-browser",
    args: ["--no-sandbox"],
  });
  const page = await browser.newPage();
  const requests = [];
  page.on("response", (res) => {
    if (res.url().startsWith(`${BASE}/api/`)) requests.push(`${res.status()} ${res.url().replace(BASE, "")}`);
  });

  // 1. Raiz redireciona para /masuk (guard: não autenticado).
  await page.goto(BASE + "/", { waitUntil: "load", timeout: 60000 });
  await page.waitForURL("**/masuk**", { timeout: 20000 }).catch(() => {});
  if (!page.url().includes("/masuk")) fail("root-redirect", `esperado /masuk, obtido ${page.url()}`);
  console.log("OK raiz -> /masuk");

  // 2. Formulário de login presente.
  await page.waitForSelector("input[type='password']", { timeout: 20000 });
  console.log("OK masuk renderizou formulário");

  // 3. Login com a senha inicial.
  await page.fill("input[type='password']", PASSWORD);
  await page.click("button[type='submit']");
  await page.waitForURL("**/dashboard**", { timeout: 30000 });
  console.log("OK login -> " + page.url());

  // 4. Dashboard renderizou o layout (navegação do dashboard).
  await page.waitForSelector("a[href*='/dashboard/']", { timeout: 30000 });
  console.log("OK dashboard layout renderizou");

  // 5. Deep link + reload em rota aninhada mantém sessão e página.
  await page.goto(BASE + "/dashboard/providers", { waitUntil: "load", timeout: 60000 });
  await page.reload({ waitUntil: "load", timeout: 60000 });
  if (!page.url().includes("/dashboard/providers")) fail("deep-link", `url final ${page.url()}`);
  await page.waitForTimeout(2000);
  const body = await page.textContent("body");
  if (/VansAI could not load|This page could not be found/.test(body)) fail("deep-link", "página de erro após reload");
  console.log("OK deep link /dashboard/providers com reload");

  // 6. Rota inexistente sob /dashboard cai no 404 da SPA (não no dashboard).
  await page.goto(BASE + "/dashboard/rota-inexistente", { waitUntil: "load", timeout: 60000 });
  await page.waitForTimeout(1500);
  const missing = await page.textContent("body");
  if (!/404|could not be found/.test(missing)) fail("spa-404", "sem 404 para rota inexistente");
  console.log("OK 404 da SPA para rota inexistente");

  console.log("--- chamadas /api via proxy ---");
  console.log(requests.slice(0, 20).join("\n"));
  await browser.close();
  console.log("SMOKE_OK");
})().catch((e) => fail("inesperado", e.message));

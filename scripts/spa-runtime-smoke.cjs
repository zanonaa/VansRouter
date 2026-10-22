// Smoke de runtime da Fase 4A: SPA Vite (20129, proxy /api→Go 20128) no
// navegador real. Valida guard, login com senha inicial, dashboard e deep link.
// Executado uma única vez contra dados descartáveis; não é teste de regressão.
// Nota: waitUntil "load" (networkidle nunca ocorre com o WebSocket do HMR).
const { chromium } = require("playwright-core");

const BASE = process.env.SAME_ORIGIN === "1" ? "http://localhost:20128" : "http://localhost:20129";
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

  // 7. CRUD de escrita no contexto autenticado da SPA (mesma origem/cookies).
  const crud = await page.evaluate(async () => {
    const json = (r) => r.json().catch(() => ({}));
    const created = await fetch("/api/keys", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: "smoke-spa" }),
    });
    const key = await json(created);
    if (!created.ok) return { step: "create", status: created.status, body: key };
    const id = key.id || key.apiKey?.id || key.key?.id;
    const list = await fetch("/api/keys").then(json);
    const found = JSON.stringify(list).includes("smoke-spa");
    const removed = id
      ? await fetch(`/api/keys/${id}`, { method: "DELETE" })
      : new Response(null, { status: 500 });
    const after = await fetch("/api/keys").then(json);
    const gone = !JSON.stringify(after).includes("smoke-spa");
    return { created: created.status, found, removed: removed.status, gone };
  });
  if (crud.created !== 200 && crud.created !== 201) fail("keys-crud", JSON.stringify(crud));
  if (!crud.found || !crud.gone || (crud.removed !== 200 && crud.removed !== 204)) fail("keys-crud", JSON.stringify(crud));
  console.log("OK CRUD /api/keys via SPA:", JSON.stringify(crud));

  // 8. Contrato de leitura grande da SPA.
  const modelsStatus = await page.evaluate(() => fetch("/api/models").then((r) => r.status));
  if (modelsStatus !== 200) fail("models", `GET /api/models -> ${modelsStatus}`);
  console.log("OK GET /api/models 200");

  // 9. SSE autenticado.
  // SAME_ORIGIN=1: EventSource direto na mesma origem (Go serve API e SPA) —
  // valida o fechamento da pendência de streaming da 4A. No modo dev-proxy,
  // o proxy do Vite prende SSE (ver vite.config.js) e validamos na origem Go.
  let sse;
  if (process.env.SAME_ORIGIN === "1") {
    sse = await page.evaluate(
      () =>
        new Promise((resolve) => {
          const es = new EventSource("/api/translator/console-logs/stream");
          const done = (result) => {
            es.close();
            resolve(result);
          };
          es.onopen = () => done({ ok: true, mode: "eventsource-same-origin" });
          es.onerror = () => done({ ok: false });
          setTimeout(() => done({ ok: es.readyState === 1 }), 8000);
        })
    );
    if (!sse.ok) fail("sse", JSON.stringify(sse));
    console.log("OK SSE mesma origem:", JSON.stringify(sse));
  } else {
    const cookies = await page.context().cookies(BASE);
    const token = cookies.find((c) => c.name === "auth_token")?.value;
    if (!token) fail("sse", "cookie auth_token ausente após login");
    sse = await new Promise((resolve) => {
      const req = require("http").request(
        new URL("/api/translator/console-logs/stream", BASE.replace("20129", "20128")),
        { headers: { Cookie: `auth_token=${token}`, Accept: "text/event-stream" }, timeout: 8000 },
        (res) => {
          if (res.statusCode !== 200 || !String(res.headers["content-type"] || "").includes("text/event-stream")) {
            resolve({ ok: false, status: res.statusCode, ct: res.headers["content-type"] });
            res.resume();
            return;
          }
          // Buffer de logs vazio não emite chunk; headers de evento + conexão
          // mantida já comprovam o stream autenticado na origem.
          resolve({ ok: true, status: res.statusCode, ct: res.headers["content-type"] });
          setTimeout(() => req.destroy(), 500);
          res.resume();
        }
      );
      req.on("timeout", () => {
        resolve({ ok: false, timeout: true });
        req.destroy();
      });
      req.on("error", () => {});
      req.end();
    });
    if (!sse.ok) fail("sse", JSON.stringify(sse));
    console.log("OK SSE na origem Go:", JSON.stringify(sse));
  }

  console.log("--- chamadas /api via proxy ---");
  console.log(requests.slice(0, 20).join("\n"));
  await browser.close();
  console.log("SMOKE_OK");
})().catch((e) => fail("inesperado", e.message));

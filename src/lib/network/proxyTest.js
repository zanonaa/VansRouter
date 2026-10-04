import { ProxyAgent, fetch as undiciFetch } from "undici";

// Connectivity check targets, tried in order until one answers.
//
// Method: GET, never HEAD. Measured against live public HTTP proxies,
// `HEAD https://google.com/` never completes (8s timeout) on proxies that
// answer `GET /generate_204` in ~3s — many open proxies mishandle HEAD and
// stall forever, which produced false "dead" verdicts for working proxies.
//
// Multiple targets: open proxies also break per-host in arbitrary ways.
// Measured examples — one proxy refuses CONNECT to www.google.com while
// answering www.gstatic.com and detectportal.firefox.com; another hangs on
// the google.com apex while answering www.google.com. A single fixed target
// misclassifies a real share of working proxies, so a check passes when ANY
// of these HTTPS endpoints (three different hosts, two different CDNs)
// returns a 2xx. The pool's real job is CONNECT + TLS to AI endpoints, so
// plain-HTTP-only reachability is deliberately not enough.
const DEFAULT_TEST_URLS = [
  "https://www.google.com/generate_204",
  "https://www.gstatic.com/generate_204",
  "https://detectportal.firefox.com/success.txt",
];
const DEFAULT_TIMEOUT_MS = 8000;

function getErrorMessage(err) {
  if (!err) return "Unknown error";
  const base = err?.message || String(err);
  const causeCode = err?.cause?.code || err?.code;
  const causeMessage = err?.cause?.message;

  if (causeMessage && causeMessage !== base) {
    return causeCode ? `${base}: ${causeMessage} (${causeCode})` : `${base}: ${causeMessage}`;
  }

  if (causeCode && !base.includes(causeCode)) {
    return `${base} (${causeCode})`;
  }

  return base;
}

function normalizeString(value) {
  if (value === undefined || value === null) return "";
  return String(value).trim();
}

async function attemptThroughProxy({ dispatcher, url, timeoutMs }) {
  const controller = new AbortController();
  const startedAt = Date.now();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const res = await undiciFetch(url, {
      method: "GET",
      dispatcher,
      signal: controller.signal,
      headers: {
        "User-Agent": "VansRouter",
      },
    });

    return {
      ok: res.ok,
      status: res.status,
      statusText: res.statusText,
      url,
      elapsedMs: Date.now() - startedAt,
    };
  } catch (err) {
    return {
      ok: false,
      status: 500,
      url,
      elapsedMs: Date.now() - startedAt,
      error: err?.name === "AbortError" ? "Proxy test timed out" : getErrorMessage(err),
    };
  } finally {
    clearTimeout(timer);
  }
}

export async function testProxyUrl({ proxyUrl, testUrl, timeoutMs } = {}) {
  const normalizedProxyUrl = normalizeString(proxyUrl);
  if (!normalizedProxyUrl) {
    return { ok: false, status: 400, error: "proxyUrl is required" };
  }

  // An explicit testUrl stays single-target — the caller chose that endpoint.
  const targets = normalizeString(testUrl) ? [normalizeString(testUrl)] : DEFAULT_TEST_URLS;
  const timeoutMsRaw = Number(timeoutMs);
  const normalizedTimeoutMs =
    Number.isFinite(timeoutMsRaw) && timeoutMsRaw > 0
      ? Math.min(timeoutMsRaw, 30000)
      : DEFAULT_TIMEOUT_MS;

  let dispatcher;

  try {
    try {
      dispatcher = new ProxyAgent({ uri: normalizedProxyUrl });
    } catch (err) {
      return {
        ok: false,
        status: 400,
        error: `Invalid proxy URL: ${err?.message || String(err)}`,
      };
    }

    const failures = [];
    for (const target of targets) {
      const result = await attemptThroughProxy({ dispatcher, url: target, timeoutMs: normalizedTimeoutMs });
      if (result.ok) return result;
      failures.push(`${new URL(target).host}: ${result.error || `status ${result.status}`}`);
    }

    return {
      ok: false,
      status: 500,
      url: targets[targets.length - 1],
      error: failures.join("; "),
    };
  } finally {
    try {
      await dispatcher?.close?.();
    } catch {
      // ignore
    }
  }
}

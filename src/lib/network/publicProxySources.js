import { PUBLIC_PROXY_ANONYMITY_LEVELS, PUBLIC_PROXY_SOURCES } from "@/shared/constants/publicProxySources.js";

/**
 * Fetch + normalize public free-proxy list feeds into proxy pool entries.
 *
 * Security posture: every upstream URL is built from a fixed host plus a
 * whitelisted path/query — no user-supplied URL ever reaches the fetch
 * layer, so the import path cannot be turned into an SSRF relay.
 */

const FREEPROXYDB_SEARCH_URL = "https://freeproxydb.com/api/proxy/search";
const VPSLAB_BASE_URL = "https://raw.githubusercontent.com/VPSLabCloud/VPSLab-Free-Proxy-List/main";

const FETCH_TIMEOUT_MS = 20000;
const VPSLAB_IP_PORT_PATTERN = /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3}):(\d{1,5})$/;

// VPSLab ships one file per protocol/SSL/anonymity cut. ssl+transparent has
// no dedicated file — it falls back to the plain SSL list (documented in the
// route response so the UI can show an honest summary).
const VPSLAB_HTTP_FILES = {
  any: { any: "http_all.txt", elite: "http_elite.txt", anonymous: "http_anonymous.txt", transparent: "http_transparent.txt" },
  ssl: { any: "http_ssl.txt", elite: "http_ssl_elite.txt", anonymous: "http_ssl_anonymous.txt", transparent: "http_ssl.txt" },
};

function isValidIp(octets) {
  return octets.every((part) => Number(part) >= 0 && Number(part) <= 255);
}

/**
 * Canonical proxy URL key for dedupe. Mirrors the batch-import parser
 * (parseProxyLine → URL.toString()): special schemes (http/https) get a
 * trailing "/", socks5 does not. Idempotent.
 */
export function normalizeProxyUrlKey(value) {
  const trimmed = typeof value === "string" ? value.trim() : "";
  if (!trimmed) return "";
  try {
    return new URL(trimmed).toString();
  } catch {
    return trimmed;
  }
}

export function resolveVpsLabFile({ protocol, anonymity, httpsOnly } = {}) {
  if (protocol === "socks5") return "socks5_all.txt";
  const sslKey = httpsOnly === true ? "ssl" : "any";
  const anonymityKey = PUBLIC_PROXY_ANONYMITY_LEVELS.includes(anonymity) ? anonymity : "any";
  return VPSLAB_HTTP_FILES[sslKey][anonymityKey];
}

function buildVpsLabUrl(options) {
  return `${VPSLAB_BASE_URL}/${resolveVpsLabFile(options)}`;
}

function parseVpsLabText(text, { protocol, count }) {
  const scheme = protocol === "socks5" ? "socks5" : "http";
  const seen = new Set();
  const entries = [];
  for (const rawLine of text.split(/\r\n|\n|\r/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith("#")) continue;
    const match = VPSLAB_IP_PORT_PATTERN.exec(line);
    if (!match) continue;
    if (!isValidIp([match[1], match[2], match[3], match[4]])) continue;
    const proxyUrl = normalizeProxyUrlKey(`${scheme}://${line}`);
    if (seen.has(proxyUrl)) continue;
    seen.add(proxyUrl);
    entries.push({ name: `VPSLab ${line}`, proxyUrl, type: "http" });
    if (entries.length >= count) break;
  }
  return entries;
}

function buildFreeProxyDbUrl({ count, protocol, country, anonymity, httpsOnly, maxSpeed } = {}) {
  const params = new URLSearchParams();
  params.set("protocol", protocol || "http");
  if (country) params.set("country", country);
  if (anonymity) params.set("anonymity", anonymity);
  if (httpsOnly === true) params.set("https", "1");
  if (maxSpeed) params.set("speed", `0,${maxSpeed}`);
  params.set("page_size", String(count));
  params.set("order_by", "check_success_count");
  params.set("order_dir", "desc");
  return `${FREEPROXYDB_SEARCH_URL}?${params.toString()}`;
}

function parseFreeProxyDbPayload(payload, { count }) {
  const rows = Array.isArray(payload?.data?.data) ? payload.data.data : [];
  const entries = [];
  const seen = new Set();
  for (const row of rows) {
    // The search endpoint returns invalid entries too — only currently
    // verified proxies may land in the pool.
    if (row?.is_valid !== 1) continue;
    const connectString = typeof row.connect_string === "string" ? row.connect_string.trim() : "";
    if (!connectString) continue;
    const scheme = connectString.split("://")[0];
    if (scheme !== "http" && scheme !== "socks5") continue;
    const proxyUrl = normalizeProxyUrlKey(connectString);
    if (!proxyUrl) continue;
    if (seen.has(proxyUrl)) continue;
    seen.add(proxyUrl);
    const hostLabel = row.port ? `${row.ip}:${row.port}` : `${row.ip}`;
    const country = typeof row.country === "string" && row.country ? ` ${row.country}` : "";
    entries.push({ name: `FreeProxyDB${country} ${hostLabel}`, proxyUrl, type: "http" });
    if (entries.length >= count) break;
  }
  return entries;
}

async function fetchUpstream(url, { sourceLabel, timeoutMs = FETCH_TIMEOUT_MS } = {}) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const response = await fetch(url, {
      signal: controller.signal,
      headers: { Accept: "application/json, text/plain, */*" },
    });
    if (!response.ok) {
      const body = await response.text().catch(() => "");
      const detail = body ? `: ${body.slice(0, 200)}` : "";
      const hint = response.status === 429
        ? ` ${sourceLabel} public rate limit reached — retry in a minute or lower the count.`
        : "";
      const error = new Error(`${sourceLabel} returned HTTP ${response.status}${detail}.${hint}`);
      error.status = response.status;
      throw error;
    }
    return response;
  } catch (err) {
    if (err?.name === "AbortError") {
      const error = new Error(`${sourceLabel} request timed out after ${timeoutMs}ms.`);
      error.status = 504;
      throw error;
    }
    throw err;
  } finally {
    clearTimeout(timer);
  }
}

export function getPublicProxySource(sourceId) {
  const source = PUBLIC_PROXY_SOURCES[sourceId];
  if (!source) {
    throw new Error(`Unknown proxy source: ${sourceId}`);
  }
  return source;
}

export async function fetchPublicProxyEntries(sourceId, options = {}) {
  if (sourceId === "vpslab") {
    const source = getPublicProxySource(sourceId);
    const response = await fetchUpstream(buildVpsLabUrl(options), { sourceLabel: source.label });
    const text = await response.text();
    return parseVpsLabText(text, options);
  }

  if (sourceId === "freeproxydb") {
    const source = getPublicProxySource(sourceId);
    const response = await fetchUpstream(buildFreeProxyDbUrl(options), { sourceLabel: source.label });
    const payload = await response.json().catch(() => {
      const error = new Error("FreeProxyDB returned a non-JSON payload.");
      error.status = 502;
      throw error;
    });
    return parseFreeProxyDbPayload(payload, options);
  }

  return getPublicProxySource(sourceId) && [];
}

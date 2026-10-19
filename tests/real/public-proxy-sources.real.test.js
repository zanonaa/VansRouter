// Live smoke against the real public proxy sources.
//   RUN_REAL=1 pnpm vitest run -c tests/vitest.config.js tests/real/public-proxy-sources.real.test.js
// FreeProxyDB public search is rate limited (~3 req/min, 10 req/h per IP) —
// this suite makes exactly one FreeProxyDB request.
import { describe, it, expect } from "vitest";
import { fetchPublicProxyEntries } from "@/lib/network/publicProxySources.js";

const RUN_REAL = process.env.RUN_REAL === "1";

describe.skipIf(!RUN_REAL)("public proxy sources (live)", () => {
  it("freeproxydb returns valid http entries with connect strings", async () => {
    const entries = await fetchPublicProxyEntries("freeproxydb", { count: 10, protocol: "http" });
    expect(entries.length).toBeGreaterThan(0);
    expect(entries.length).toBeLessThanOrEqual(10);
    for (const entry of entries) {
      expect(entry.proxyUrl).toMatch(/^http:\/\//);
      expect(entry.name).toMatch(/^FreeProxyDB /);
      expect(entry.type).toBe("http");
    }
  });

  it("vpslab http_all parses ip:port lines as http proxies", async () => {
    const entries = await fetchPublicProxyEntries("vpslab", { protocol: "http", count: 10 });
    expect(entries.length).toBeGreaterThan(0);
    for (const entry of entries) {
      expect(entry.proxyUrl).toMatch(/^http:\/\/\d+\.\d+\.\d+\.\d+/);
      expect(entry.name).toMatch(/^VPSLab /);
    }
  });

  it("vpslab socks5_all prefixes the socks5 scheme", async () => {
    const entries = await fetchPublicProxyEntries("vpslab", { protocol: "socks5", count: 10 });
    expect(entries.length).toBeGreaterThan(0);
    for (const entry of entries) {
      expect(entry.proxyUrl).toMatch(/^socks5:\/\//);
    }
  });
});

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const { ProxyAgent, fetch } = vi.hoisted(() => ({
  ProxyAgent: vi.fn(() => ({ close: async () => {} })),
  fetch: vi.fn(),
}));

vi.mock("undici", () => ({ ProxyAgent, fetch }));

import { testProxyUrl } from "@/lib/network/proxyTest.js";

const ok204 = { ok: true, status: 204, statusText: "No Content" };

beforeEach(() => {
  fetch.mockResolvedValue(ok204);
});

afterEach(() => {
  vi.clearAllMocks();
});

describe("testProxyUrl", () => {
  it("uses GET on the first default target when it answers", async () => {
    const result = await testProxyUrl({ proxyUrl: "http://1.2.3.4:8080" });

    expect(fetch).toHaveBeenCalledTimes(1);
    const [url, options] = fetch.mock.calls[0];
    expect(url).toBe("https://www.google.com/generate_204");
    expect(options.method).toBe("GET");
    expect(result).toMatchObject({ ok: true, status: 204 });
  });

  it("falls back to the next target when the first is refused — per-host breakage is real on open proxies", async () => {
    // Measured case: a proxy that refuses CONNECT to www.google.com while
    // answering www.gstatic.com and detectportal.firefox.com.
    const connectRefused = new TypeError("Request was cancelled.");
    fetch.mockRejectedValueOnce(connectRefused).mockResolvedValueOnce(ok204);

    const result = await testProxyUrl({ proxyUrl: "http://1.2.3.4:8080" });

    expect(fetch).toHaveBeenCalledTimes(2);
    expect(fetch.mock.calls[0][0]).toBe("https://www.google.com/generate_204");
    expect(fetch.mock.calls[1][0]).toBe("https://www.gstatic.com/generate_204");
    expect(result).toMatchObject({ ok: true, url: "https://www.gstatic.com/generate_204" });
  });

  it("is dead only after every target fails, with the per-host reasons", async () => {
    fetch.mockRejectedValue(new TypeError("fetch failed"));

    const result = await testProxyUrl({ proxyUrl: "http://1.2.3.4:8080" });

    expect(fetch).toHaveBeenCalledTimes(3);
    expect(result.ok).toBe(false);
    expect(result.error).toContain("www.google.com: fetch failed");
    expect(result.error).toContain("www.gstatic.com: fetch failed");
    expect(result.error).toContain("detectportal.firefox.com: fetch failed");
  });

  it("keeps an explicit testUrl single-target", async () => {
    fetch.mockRejectedValue(new TypeError("fetch failed"));

    const result = await testProxyUrl({ proxyUrl: "http://1.2.3.4:8080", testUrl: "http://httpbin.org/ip" });

    expect(fetch).toHaveBeenCalledTimes(1);
    expect(fetch.mock.calls[0][0]).toBe("http://httpbin.org/ip");
    expect(result.ok).toBe(false);
  });

  it("reports a timeout as a failed attempt, not a thrown error", async () => {
    const abortError = new Error("This operation was aborted");
    abortError.name = "AbortError";
    fetch.mockRejectedValue(abortError);

    const result = await testProxyUrl({ proxyUrl: "http://1.2.3.4:8080" });

    expect(result.ok).toBe(false);
    expect(result.error).toContain("Proxy test timed out");
  });

  it("requires a proxyUrl", async () => {
    const result = await testProxyUrl({});

    expect(result).toMatchObject({ ok: false, status: 400 });
    expect(fetch).not.toHaveBeenCalled();
  });
});

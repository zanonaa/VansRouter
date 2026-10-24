import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchAppVersion } from "../../src/shared/hooks/useAppVersion.js";

afterEach(() => vi.unstubAllGlobals());

describe("running application version", () => {
  it("uses the backend build even without an available update", async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ currentVersion: "2.0.36", hasUpdate: false }),
    });
    vi.stubGlobal("fetch", fetchMock);
    expect(await fetchAppVersion()).toEqual({ currentVersion: "2.0.36", hasUpdate: false });
    expect(fetchMock).toHaveBeenCalledWith("/api/version", { cache: "no-store" });
  });

  it("preserves the runtime override and update metadata", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ currentVersion: "custom-dev", hasUpdate: true, latestVersion: "3.0.0" }),
    }));
    expect((await fetchAppVersion()).currentVersion).toBe("custom-dev");
  });

  it("does not treat HTTP errors or missing versions as release 2.0.0", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false }));
    await expect(fetchAppVersion()).rejects.toThrow();
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) }));
    await expect(fetchAppVersion()).rejects.toThrow("Missing running version");
  });
});

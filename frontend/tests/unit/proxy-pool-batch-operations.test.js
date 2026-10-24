import { describe, expect, it, vi } from "vitest";
import { countBatchResults, dedupeProxyEntries, runProxyPoolBatch } from "../../src/app/(dashboard)/dashboard/proxy-pools/batchOperations.js";

describe("proxy pool batch operations", () => {
  it("deduplicates entries against existing and earlier input entries", () => {
    const existing = new Set(["http://existing:1/|||"]);
    const entries = [
      { proxyUrl: "http://existing:1/" },
      { proxyUrl: "http://new:2/" },
      { proxyUrl: "http://new:2/" },
    ];

    const result = dedupeProxyEntries(entries, existing);

    expect(result.accepted).toEqual([{ proxyUrl: "http://new:2/" }]);
    expect(result.skipped).toBe(2);
  });

  it("reports progress and preserves partial failures", async () => {
    const progress = [];
    const results = await runProxyPoolBatch([1, 2, 3], async (value) => {
      if (value === 2) throw new Error("failed");
      return "ok";
    }, (current, total) => progress.push([current, total]));

    expect(results).toEqual(["ok", "fail", "ok"]);
    expect(progress).toEqual([[1, 3], [2, 3], [3, 3]]);
    expect(countBatchResults(results)).toEqual({ ok: 2, fail: 1 });
  });

  it("does not call progress when there are no items", async () => {
    const progress = vi.fn();
    await expect(runProxyPoolBatch([], vi.fn(), progress)).resolves.toEqual([]);
    expect(progress).not.toHaveBeenCalled();
  });
});

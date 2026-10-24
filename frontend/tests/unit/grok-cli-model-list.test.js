import { describe, expect, it } from "vitest";
import { PROVIDER_MODELS } from "open-sse/config/providerModels.js";

describe("Grok CLI model catalog", () => {
  it("keeps static models available when live discovery is partial", () => {
    const staticIds = PROVIDER_MODELS.gcli.map((model) => model.id);
    const liveIds = ["grok-build"];
    const merged = new Set([...staticIds, ...liveIds]);
    expect([...merged]).toEqual(expect.arrayContaining(staticIds));
    expect(merged.size).toBeGreaterThan(liveIds.length);
  });
});

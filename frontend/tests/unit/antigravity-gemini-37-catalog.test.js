import { describe, expect, it } from "vitest";
import antigravity from "../../open-sse/providers/registry/antigravity.js";
import { getCapabilitiesForModel } from "../../open-sse/providers/capabilities.js";
import { resolveAntigravityUpstreamModel } from "../../open-sse/config/providerModels.js";

describe("Antigravity Gemini 3.7 catalog", () => {
  it("exposes all three explicit tiers without the plain alias", () => {
    expect(antigravity.models.map(({ id }) => id)).toEqual(expect.arrayContaining([
      "gemini-3.7-flash-high",
      "gemini-3.7-flash-medium",
      "gemini-3.7-flash-low",
    ]));
    expect(antigravity.models.map(({ id }) => id)).not.toContain("gemini-3.7-flash");
  });

  it("resolves upstream capability contract", () => {
    expect(getCapabilitiesForModel("antigravity", "gemini-3.7-flash-medium")).toMatchObject({
      vision: true,
      audioInput: true,
      videoInput: true,
      reasoning: true,
      search: true,
      thinkingFormat: "gemini-level",
      contextWindow: 1048576,
      maxOutput: 65536,
    });
  });

  it.each(["low", "medium", "high"])("maps %s to the tiered upstream model", (tier) => {
    expect(resolveAntigravityUpstreamModel(`gemini-3.7-flash-${tier}`)).toBe("gemini-3.7-flash-tiered");
  });
});

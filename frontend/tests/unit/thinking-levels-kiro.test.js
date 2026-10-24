import { describe, expect, it } from "vitest";
import { applyKiroThinkingOverride, resolveKiroModelIntent } from "../../open-sse/config/kiroConstants.js";
import { getThinkingLevels } from "../../open-sse/providers/thinkingLevels.js";
import { applyThinking } from "../../open-sse/translator/concerns/thinkingUnified.js";
import { buildKiroAdditionalModelRequestFieldsForModel } from "../../open-sse/config/kiroConstants.js";

describe("Kiro model(level) suffix", () => {
  it("strips suffix before synthetic Kiro variants", () => {
    expect(resolveKiroModelIntent("claude-opus-5(high)")).toMatchObject({
      model: "claude-opus-5",
      upstream: "claude-opus-5",
      thinking: false,
      thinkingOverride: { mode: "level", level: "high" },
    });
  });

  it("maps numeric suffix to enabled budget", () => {
    const intent = resolveKiroModelIntent("claude-opus-5(8192)");
    expect(applyKiroThinkingOverride({}, intent.thinkingOverride)).toEqual({
      thinking: { type: "enabled", budget_tokens: 8192 },
    });
  });

  it("omits xhigh on 4.6 models (upstream rejects it there)", () => {
    for (const model of ["claude-opus-4.6", "claude-opus-4-6", "claude-sonnet-4.6"]) {
      expect(getThinkingLevels("kiro", model)).not.toContain("xhigh");
      expect(getThinkingLevels("kiro", model)).toContain("max");
    }
  });

  it("passes xhigh/max through on the wire for 4.7+, clamps xhigh on 4.6", () => {
    const xhigh = { output_config: { effort: "xhigh" } };
    expect(buildKiroAdditionalModelRequestFieldsForModel(xhigh, "claude-sonnet-5")?.output_config?.effort).toBe("xhigh");
    expect(buildKiroAdditionalModelRequestFieldsForModel({ output_config: { effort: "max" } }, "claude-opus-4.6")?.output_config?.effort).toBe("max");
    expect(buildKiroAdditionalModelRequestFieldsForModel(xhigh, "claude-opus-4.6")?.output_config?.effort).toBe("high");
    // Anthropic-wire path: suffix override sends real xhigh on 4.7+, high on 4.6.
    expect(applyThinking("claude", "claude-opus-5.5(xhigh)", { messages: [] }, "claude").output_config?.effort).toBe("xhigh");
    expect(applyThinking("claude", "claude-opus-4.6(xhigh)", { messages: [] }, "claude").output_config?.effort).toBe("high");
  });
});

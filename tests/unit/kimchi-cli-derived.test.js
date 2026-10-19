import { describe, it, expect } from "vitest";

// Derived from the provider's documented model and quota behavior.
// The official CLI catalog (https://models.dev/api.json) lists kimi-k2.6 output limit
// as 16384 tokens, so 9router now matches that ceiling for NVIDIA-hosted k2.6 while
// still protecting against the >=~32k degeneration observed in the original audit.
describe("Kimchi CLI-derived fixes", () => {
  async function transformedMaxTokens(model, bodyExtra) {
    const { DefaultExecutor } = await import("../../open-sse/executors/default.js");
    const executor = new DefaultExecutor("nvidia");
    const out = executor.transformRequest(model, { messages: [{ role: "user", content: "hello" }], ...bodyExtra });
    return out.max_tokens;
  }

  it("sets NVIDIA kimi-k2.6 max_tokens ceiling to the CLI-published 16384", async () => {
    expect(await transformedMaxTokens("moonshotai/kimi-k2.6", { max_tokens: 64000 })).toBe(16384);
  });

  it("does not change NVIDIA kimi-k2.6 max_tokens at or below 16384", async () => {
    expect(await transformedMaxTokens("moonshotai/kimi-k2.6", { max_tokens: 16384 })).toBe(16384);
    expect(await transformedMaxTokens("moonshotai/kimi-k2.6", { max_tokens: 12000 })).toBe(12000);
  });

  it("still clamps NVIDIA kimi-k2.7 to the conservative 8192 ceiling", async () => {
    expect(await transformedMaxTokens("moonshotai/kimi-k2.7", { max_tokens: 64000 })).toBe(8192);
  });
});

import { describe, it, expect } from "vitest";

// 9Router clamps max_tokens to a safe ceiling for NVIDIA-hosted Kimi models — empirically
// a very large value (>=~32k) makes the model degenerate/loop. Smaller values pass through;
// it never INJECTS a value when the client omits it. The clamp lives in
// DefaultExecutor.transformRequest (matching the gateway's default-executor request path), which is the body
// BaseExecutor.execute stringifies and sends upstream.
//
// CLI-derived refinement: the official Kimi CLI catalog (https://models.dev/api.json)
// lists an output limit of 16384 for kimi-k2.6, so its ceiling was raised from 8192.
// k2.7 variants remain at 8192 pending a similarly authoritative limit.
describe("Kimi NVIDIA max_tokens clamp", () => {
  async function transformedMaxTokens(model, bodyExtra) {
    const { DefaultExecutor } = await import("../../open-sse/executors/default.js");
    const executor = new DefaultExecutor("nvidia");
    const out = executor.transformRequest(model, { messages: [{ role: "user", content: "hello" }], ...bodyExtra });
    return out.max_tokens;
  }

  it("clamps a large max_tokens (64000) for k2.6 to the CLI-published 16384 ceiling", async () => {
    expect(await transformedMaxTokens("moonshotai/kimi-k2.6", { max_tokens: 64000 })).toBe(16384);
  });

  it("honors a small max_tokens (2048) for k2.6 unchanged", async () => {
    expect(await transformedMaxTokens("moonshotai/kimi-k2.6", { max_tokens: 2048 })).toBe(2048);
  });

  it("does NOT inject max_tokens when client omits it", async () => {
    expect(await transformedMaxTokens("moonshotai/kimi-k2.6", {})).toBeUndefined();
  });

  it("does NOT clamp non-Kimi NVIDIA models", async () => {
    expect(await transformedMaxTokens("meta/llama-3.1-8b-instruct", { max_tokens: 64000 })).toBe(64000);
  });

  it("clamps k2.7 to the original 8192 ceiling", async () => {
    expect(await transformedMaxTokens("moonshotai/kimi-k2.7", { max_tokens: 50000 })).toBe(8192);
  });

  it("leaves k2.7 values below 8192 unchanged", async () => {
    expect(await transformedMaxTokens("moonshotai/kimi-k2.7", { max_tokens: 4096 })).toBe(4096);
  });
});

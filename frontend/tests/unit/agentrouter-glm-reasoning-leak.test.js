// AgentRouter GLM-5.2 reasoning leak (#2071 follow-up)
//
// AgentRouter uses Claude-format transport but proxies to GLM-5.2, which
// defaults thinking ON when no reasoning config is sent. The previous fix
// defaulted *all* zai-format providers to disabled thinking, which broke
// native glm/Z.ai. This regression test scopes the behavior to agentrouter
// only and verifies the translated Claude-format body carries valid thinking
// config rather than an invalid OpenAI `reasoning_effort` field.
import { describe, expect, it } from "vitest";
import { applyThinking } from "../../open-sse/translator/concerns/thinkingUnified.js";

describe("agentrouter GLM-5.2 reasoning leak prevention", () => {
  it("disables thinking by default when client does not request reasoning", () => {
    const body = { messages: [{ role: "user", content: "hi" }] };
    applyThinking("claude", "glm-5.2", body, "agentrouter");
    // Claude-format transport must receive Claude-native thinking config.
    expect(body.thinking).toEqual({ type: "disabled" });
    // OpenAI-style field must not leak onto a Claude-format body.
    expect(body.reasoning_effort).toBeUndefined();
    expect(body.enable_thinking).toBeUndefined();
  });

  it("keeps thinking enabled when client explicitly requests reasoning", () => {
    const body = { messages: [{ role: "user", content: "hi" }], reasoning_effort: "high" };
    applyThinking("claude", "glm-5.2", body, "agentrouter");
    expect(body.thinking).toBeDefined();
    expect(body.thinking.type).toBe("enabled");
    expect(body.reasoning_effort).toBeUndefined();
  });

  it("respects explicit reasoning off", () => {
    const body = { messages: [{ role: "user", content: "hi" }], reasoning_effort: "none" };
    applyThinking("claude", "glm-5.2", body, "agentrouter");
    expect(body.thinking).toEqual({ type: "disabled" });
  });

  it("does NOT disable thinking for native glm provider when no intent is present", () => {
    const body = { messages: [{ role: "user", content: "hi" }] };
    applyThinking("claude", "glm-5.2", body, "glm");
    // Native glm/Z.ai should remain untouched when client didn't ask.
    expect(body.thinking).toBeUndefined();
    expect(body.reasoning_effort).toBeUndefined();
    expect(body.enable_thinking).toBeUndefined();
  });
});

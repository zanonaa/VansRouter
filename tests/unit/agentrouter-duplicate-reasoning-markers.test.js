// Regression: duplicate reasoning markers on Claude-format transports
// (e.g. AgentRouter + GLM-5.2/GPT-5.5, and native Claude upstreams).
//
// claudeToOpenAIResponse used to emit BOTH `reasoning_content` deltas AND
// literal `<think>…</think>` content tags for the same thinking block.
// OpenAI clients that already understand reasoning_content (e.g. OpenCode)
// captured the field as `thought` but still received `<think>` and
// `</think>` as plain content, leaking reasoning markers into the chat
// surface. The assertions below lock the provider-facing behavior.
//
// History: this fork first fixed it partially, by wrapping only for native
// Claude models. Upstream's 5d2cfbf3 then removed the markers entirely
// (#3399, #4199) — the pair always arrived empty, since the thinking text
// travels in reasoning_content — and that supersedes the gate here. No
// `<think>` marker is emitted for any model now.
import { describe, expect, it } from "vitest";
import { claudeToOpenAIResponse } from "../../open-sse/translator/response/claude-to-openai.js";

function makeState(model) {
  return { messageId: "msg_1", model, toolCallIndex: 0, toolCalls: new Map() };
}

describe("claude-to-openai: reasoning marker wrapping", () => {
  it("emits reasoning_content but NO <think>/</think> for GLM-5.2 (agentrouter)", () => {
    const state = makeState("glm-5.2");

    // content_block_start with THINKING → must NOT emit <think>
    const start = claudeToOpenAIResponse({
      type: "content_block_start",
      index: 0,
      content_block: { type: "thinking" }
    }, state);
    expect(start).toBeNull();

    // content_block_delta with thinking_delta → reasoning_content only
    const delta = claudeToOpenAIResponse({
      type: "content_block_delta",
      index: 0,
      delta: { type: "thinking_delta", thinking: "step 1" }
    }, state);
    expect(delta).toHaveLength(1);
    expect(delta[0].choices[0].delta.reasoning_content).toBe("step 1");
    expect(delta[0].choices[0].delta.content).toBeUndefined();

    // content_block_stop → must NOT emit </think>
    const stop = claudeToOpenAIResponse({
      type: "content_block_stop",
      index: 0
    }, state);
    expect(stop).toBeNull();
  });

  it("emits reasoning_content but NO <think>/</think> for GPT-5.5 (agentrouter)", () => {
    const state = makeState("gpt-5.5");

    const start = claudeToOpenAIResponse({
      type: "content_block_start",
      index: 0,
      content_block: { type: "thinking" }
    }, state);
    expect(start).toBeNull();

    const delta = claudeToOpenAIResponse({
      type: "content_block_delta",
      index: 0,
      delta: { type: "thinking_delta", thinking: "step" }
    }, state);
    expect(delta[0].choices[0].delta.reasoning_content).toBe("step");
    expect(delta[0].choices[0].delta.content).toBeUndefined();

    const stop = claudeToOpenAIResponse({ type: "content_block_stop", index: 0 }, state);
    expect(stop).toBeNull();
  });

  it("emits reasoning_content but NO <think>/</think> for Claude models either", () => {
    // Upstream #3399/#4199: the markers always arrived empty and adjacent to the
    // reasoning_content that carried the actual text, so clients rendered a bare
    // "<think></think>" above every answer. This fork previously kept them for
    // Claude models "for backward compat"; upstream's fix supersedes that, and
    // no information is lost because the thinking text itself travels in
    // reasoning_content (asserted just below).
    const state = makeState("claude-opus-4-6");

    const start = claudeToOpenAIResponse({
      type: "content_block_start",
      index: 0,
      content_block: { type: "thinking" }
    }, state);
    expect(start).toBeNull();

    const delta = claudeToOpenAIResponse({
      type: "content_block_delta",
      index: 0,
      delta: { type: "thinking_delta", thinking: "step 1" }
    }, state);
    expect(delta[0].choices[0].delta.reasoning_content).toBe("step 1");
    expect(delta[0].choices[0].delta.content).toBeUndefined();

    const stop = claudeToOpenAIResponse({ type: "content_block_stop", index: 0 }, state);
    expect(stop).toBeNull();
  });

  it("emits no markers for a model whose name merely contains 'claude'", () => {
    // The old lowercase-includes("claude") heuristic made this case wrap. With
    // markers gone entirely the heuristic is moot: nothing wraps, ever.
    const state = makeState("claude-replica-glm");
    const start = claudeToOpenAIResponse({
      type: "content_block_start",
      index: 0,
      content_block: { type: "thinking" }
    }, state);
    expect(start).toBeNull();
  });

  it("tolerates missing state.model without throwing", () => {
    const state = { messageId: "msg_1", toolCallIndex: 0, toolCalls: new Map() };
    expect(() => claudeToOpenAIResponse({
      type: "content_block_start",
      index: 0,
      content_block: { type: "thinking" }
    }, state)).not.toThrow();
  });
});

// Tests for Accept header negotiation.
// Rule: client sending Accept: text/event-stream WITHOUT explicit stream=false
// in body is treated as stream=true. Do NOT override explicit stream=false.
import { describe, it, expect } from "vitest";

// Mirror the negotiation logic from handleChat
function negotiateStream(body, acceptHeader) {
  const result = { ...body };
  if (acceptHeader.includes("text/event-stream") && result.stream === undefined) {
    result.stream = true;
  }
  return result;
}

describe("Accept header negotiation", () => {
  it("sets stream=true when Accept: text/event-stream and body.stream is undefined", () => {
    const body = negotiateStream({ model: "test" }, "text/event-stream");
    expect(body.stream).toBe(true);
  });

  it("does NOT override explicit stream=false", () => {
    const body = negotiateStream({ model: "test", stream: false }, "text/event-stream");
    expect(body.stream).toBe(false);
  });

  it("does NOT override explicit stream=true", () => {
    const body = negotiateStream({ model: "test", stream: true }, "text/event-stream");
    expect(body.stream).toBe(true);
  });

  it("does NOT set stream when Accept header is absent", () => {
    const body = negotiateStream({ model: "test" }, "");
    expect(body.stream).toBeUndefined();
  });

  it("does NOT set stream when Accept header is application/json", () => {
    const body = negotiateStream({ model: "test" }, "application/json");
    expect(body.stream).toBeUndefined();
  });

  it("handles Accept header with multiple media types including text/event-stream", () => {
    const body = negotiateStream({ model: "test" }, "text/event-stream, application/json");
    expect(body.stream).toBe(true);
  });

  it("handles Accept header with q-parameters", () => {
    const body = negotiateStream({ model: "test" }, "text/event-stream;q=0.9, application/json;q=0.1");
    expect(body.stream).toBe(true);
  });

  it("does NOT set stream when body.stream is null (explicit)", () => {
    // null is an explicit value, not undefined
    const body = negotiateStream({ model: "test", stream: null }, "text/event-stream");
    expect(body.stream).toBe(null);
  });

  it("preserves other body fields", () => {
    const body = negotiateStream({ model: "gpt-4", messages: [{ role: "user", content: "hi" }] }, "text/event-stream");
    expect(body.model).toBe("gpt-4");
    expect(body.messages).toHaveLength(1);
    expect(body.stream).toBe(true);
  });
});

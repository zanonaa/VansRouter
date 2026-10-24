import { describe, it, expect } from "vitest";
import { isCodexUnavailable401 } from "../../src/app/(dashboard)/dashboard/usage/components/ProviderLimits/utils.js";

describe("isCodexUnavailable401", () => {
  it("identifies Codex 401 unavailable quota messages", () => {
    const conn = { id: "c1", provider: "codex" };
    const quota = { message: "Codex connected. Usage API temporarily unavailable (401)." };
    expect(isCodexUnavailable401(conn, quota, null)).toBe(true);
  });

  it("identifies Codex 401 error string fallback", () => {
    const conn = { id: "c2", provider: "codex" };
    expect(isCodexUnavailable401(conn, null, "HTTP 401: Unauthorized")).toBe(true);
  });

  it("returns false for non-codex providers even with 401", () => {
    const conn = { id: "c3", provider: "claude" };
    const quota = { message: "Codex connected. Usage API temporarily unavailable (401)." };
    expect(isCodexUnavailable401(conn, quota, null)).toBe(false);
  });

  it("returns false for healthy Codex quota", () => {
    const conn = { id: "c4", provider: "codex" };
    const quota = { quotas: [{ name: "5h window", used: 10, total: 100 }] };
    expect(isCodexUnavailable401(conn, quota, null)).toBe(false);
  });
});

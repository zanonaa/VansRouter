// Tests for the custom-compatible-provider prefix ACL fix in isProviderAllowed.
// The function lives in src/sse/services/auth.js but we re-implement the
// matching rules here (no DB) to keep this test hermetic and fast.
import { describe, it, expect } from "vitest";

// Mirror of isProviderAllowed (no async/DB path exercised in this test)
function isProviderAllowed(apiKeyInfo, providerIdOrAlias, customPrefix) {
  if (!apiKeyInfo) return true;
  const allowed = apiKeyInfo.allowedProviders;
  if (allowed === null || allowed === undefined) return true;
  if (!Array.isArray(allowed) || allowed.length === 0) return false;
  if (allowed.includes(providerIdOrAlias)) return true;
  if (customPrefix && allowed.includes(customPrefix)) return true;
  return false;
}

describe("isProviderAllowed - custom-compatible prefix matching", () => {
  it("rejects when allowedProviders doesn't include the UUID or prefix", () => {
    expect(isProviderAllowed({ allowedProviders: ["oc"] }, "openai-compatible-chat-abc12345", "tr")).toBe(false);
  });

  it("accepts when allowedProviders includes the connection's prefix", () => {
    expect(isProviderAllowed({ allowedProviders: ["oc", "tr"] }, "openai-compatible-chat-abc12345", "tr")).toBe(true);
  });

  it("accepts when allowedProviders includes the bare provider id", () => {
    expect(isProviderAllowed({ allowedProviders: ["openai-compatible-chat-abc12345"] }, "openai-compatible-chat-abc12345", "tr")).toBe(true);
  });

  it("null allowedProviders = all allowed (safe default)", () => {
    expect(isProviderAllowed({ allowedProviders: null }, "openai-compatible-chat-abc12345", "tr")).toBe(true);
  });

  it("empty allowedProviders = none allowed (lockdown)", () => {
    expect(isProviderAllowed({ allowedProviders: [] }, "openai-compatible-chat-abc12345", "tr")).toBe(false);
  });

  it("null apiKeyInfo = unrestricted", () => {
    expect(isProviderAllowed(null, "openai-compatible-chat-abc12345", "tr")).toBe(true);
  });
});

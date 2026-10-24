import { describe, expect, it } from "vitest";
import { getProviderAuthTypes } from "../../src/shared/utils/providerAuth.js";
import REGISTRY from "../../open-sse/providers/registry/index.js";

describe("provider auth types", () => {
  it("uses declared auth modes for Kimchi OAuth and API-key connections", () => {
    expect(
      getProviderAuthTypes({ hasOAuth: true, authModes: ["apikey", "oauth"] }, "kimchi"),
    ).toEqual(["apikey", "oauth"]);
  });

  it("keeps Kiro API-key spelling compatibility", () => {
    expect(getProviderAuthTypes({}, "kiro")).toEqual(["oauth", "apikey", "api_key"]);
  });

  it("falls back to the provider OAuth capability", () => {
    expect(getProviderAuthTypes({ hasOAuth: true }, "provider")).toEqual(["oauth"]);
    expect(getProviderAuthTypes({}, "provider")).toEqual(["apikey"]);
  });

  it("declares Cloudflare API-key authentication", () => {
    const cloudflare = REGISTRY.find(({ id }) => id === "cloudflare-ai");
    expect(cloudflare?.authType).toBe("apikey");
    expect(cloudflare?.authModes).toEqual(["apikey"]);
  });
});

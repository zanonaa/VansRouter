import { describe, it, expect } from "vitest";
import { getProviderIconSrc, resolveProviderIconId } from "../../src/shared/utils/providerIcon.js";

describe("providerIcon", () => {
  it("resolves and maps icons correctly including aliases", () => {
    expect(getProviderIconSrc("perplexity-agent")).toBe("/providers/perplexity.webp");
    expect(getProviderIconSrc("perplexing")).toBe("/providers/perplexity.webp");
    expect(getProviderIconSrc("kilo")).toBe("/providers/kilocode.webp");
    expect(getProviderIconSrc("kilo-gateway")).toBe("/providers/kilocode.webp");
    expect(getProviderIconSrc("codebuddy-intl")).toBe("/providers/codebuddy-cn.webp");
    expect(getProviderIconSrc("alims-intl")).toBe("/providers/alicode-intl.webp");

    // Normal fallback
    expect(getProviderIconSrc("unknown-provider")).toBe("/providers/unknown-provider.webp");
  });
});

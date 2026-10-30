import { getProviderIconSrc } from "../../src/shared/utils/providerIcon";
import { expect, it } from "vitest";

it("custom provider ids never request an icon", () => {
  expect(getProviderIconSrc("openai-compatible-chat-2ab1e1b8-5893-4779-b316-49b59514e296")).toBeNull();
  expect(getProviderIconSrc("anthropic-compatible-abc123")).toBeNull();
  expect(getProviderIconSrc("custom-embedding-x")).toBeNull();
  // known providers keep working
  expect(getProviderIconSrc("grok-cli")).toBe("/providers/grok-cli.png");
  expect(getProviderIconSrc("anthropic")).toBe("/providers/anthropic.webp");
});

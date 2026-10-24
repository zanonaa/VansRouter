import { describe, it, expect } from "vitest";
import { computeGroupedModels } from "@/shared/components/modelSelectUtils.js";

const CUSTOM_ID = "openai-compatible-chat-abc12345";

const baseAllProviders = {};

describe("computeGroupedModels - activeProviders param (WIP contract)", () => {
  it("uses activeProviders connection (not filteredActiveProviders) for displayName on custom providers", () => {
    const groups = computeGroupedModels({
      filteredActiveProviders: [{ provider: CUSTOM_ID, name: "FilteredName" }],
      activeProviders: [{ provider: CUSTOM_ID, name: "ActiveName", providerSpecificData: { prefix: "ap" } }],
      kindFilter: null,
      providerNodes: [],
      customModels: [],
      disabledModels: {},
      modelAliases: {},
      allProviders: baseAllProviders,
    });

    expect(groups[CUSTOM_ID]).toBeDefined();
    expect(groups[CUSTOM_ID].name).toBe("ActiveName");
    expect(groups[CUSTOM_ID].alias).toBe("ap");
    expect(groups[CUSTOM_ID].isCustom).toBe(true);
  });

  it("falls back to providerNodes when activeProviders has no matching connection", () => {
    const groups = computeGroupedModels({
      filteredActiveProviders: [{ provider: CUSTOM_ID, name: "FilteredName" }],
      activeProviders: [],
      kindFilter: null,
      providerNodes: [{ id: CUSTOM_ID, name: "NodeName", prefix: "np" }],
      customModels: [],
      disabledModels: {},
      modelAliases: {},
      allProviders: baseAllProviders,
    });

    expect(groups[CUSTOM_ID].name).toBe("NodeName");
    expect(groups[CUSTOM_ID].alias).toBe("np");
  });

  it("falls back to providerId when neither activeProviders nor providerNodes has the connection", () => {
    const groups = computeGroupedModels({
      filteredActiveProviders: [{ provider: CUSTOM_ID }],
      activeProviders: [],
      kindFilter: null,
      providerNodes: [],
      customModels: [],
      disabledModels: {},
      modelAliases: {},
      allProviders: baseAllProviders,
    });

    expect(groups[CUSTOM_ID]).toBeDefined();
    expect(groups[CUSTOM_ID].alias).toBe(CUSTOM_ID);
    expect(groups[CUSTOM_ID].models[0].isPlaceholder).toBe(true);
  });

  it("uses connection.prefix from activeProviders to compose model value", () => {
    const groups = computeGroupedModels({
      filteredActiveProviders: [{ provider: CUSTOM_ID }],
      activeProviders: [{ provider: CUSTOM_ID, name: "n", providerSpecificData: { prefix: "myprefix" } }],
      kindFilter: null,
      providerNodes: [],
      customModels: [],
      disabledModels: {},
      modelAliases: { alias1: `${CUSTOM_ID}/gpt-4` },
      allProviders: baseAllProviders,
    });

    expect(groups[CUSTOM_ID].models[0].value).toBe("myprefix/gpt-4");
  });

  it("shows only video models for the video selector", () => {
    const groups = computeGroupedModels({
      filteredActiveProviders: [{ provider: "openrouter" }],
      activeProviders: [{ provider: "openrouter" }],
      kindFilter: "video",
      providerNodes: [],
      customModels: [],
      disabledModels: {},
      modelAliases: {},
      allProviders: { openrouter: { name: "OpenRouter", color: "#666" } },
    });

    expect(groups.openrouter.models.length).toBeGreaterThan(0);
    expect(groups.openrouter.models.every((model) => model.kind === "video")).toBe(true);
    expect(groups.openrouter.models.map((model) => model.id)).toContain("google/veo-3.1");
  });
});

import { getModelsByProviderId, getModelKind } from "@/shared/constants/models";
import {
  OAUTH_PROVIDERS,
  APIKEY_PROVIDERS,
  FREE_PROVIDERS,
  FREE_TIER_PROVIDERS,
  AI_PROVIDERS,
  isOpenAICompatibleProvider,
  isAnthropicCompatibleProvider,
  getProviderAlias,
} from "@/shared/constants/providers";

const PROVIDER_ORDER = [
  ...Object.keys(OAUTH_PROVIDERS),
  ...Object.keys(FREE_PROVIDERS),
  ...Object.keys(FREE_TIER_PROVIDERS),
  ...Object.keys(APIKEY_PROVIDERS),
];

const NO_AUTH_PROVIDER_IDS = Object.keys(FREE_PROVIDERS).filter((id) => FREE_PROVIDERS[id].noAuth);

export function computeGroupedModels({
  filteredActiveProviders,
  activeProviders,
  kindFilter,
  providerNodes,
  customModels,
  disabledModels,
  modelAliases,
  allProviders,
  cursorModels = [],
}) {
  const groups = {};

  const PROVIDER_AS_MODEL_KINDS = new Set(["webSearch", "webFetch"]);
  const TYPED_KINDS = new Set(["image", "tts", "stt", "embedding", "imageToText", "video"]);
  const ALLOW_PROVIDER_FALLBACK_KINDS = new Set(["tts", "image", "webFetch"]);

  const filterByKind = (models) => {
    if (!kindFilter) return models.filter((m) => m.isPlaceholder || m.isCustom || !getModelKind(m) || getModelKind(m) === "llm");
    if (!TYPED_KINDS.has(kindFilter)) return models;
    return models.filter((m) => m.isPlaceholder || getModelKind(m) === kindFilter);
  };

  const activeConnectionIds = filteredActiveProviders.map((p) => p.provider);

  const noAuthIds = kindFilter
    ? NO_AUTH_PROVIDER_IDS.filter((id) => (AI_PROVIDERS[id]?.serviceKinds || ["llm"]).includes(kindFilter))
    : NO_AUTH_PROVIDER_IDS;

  const providerIdsToShow = new Set([...activeConnectionIds, ...noAuthIds]);

  const sortedProviderIds = [...providerIdsToShow].toSorted((a, b) => {
    const indexA = PROVIDER_ORDER.indexOf(a);
    const indexB = PROVIDER_ORDER.indexOf(b);
    return (indexA === -1 ? 999 : indexA) - (indexB === -1 ? 999 : indexB);
  });

  sortedProviderIds.forEach((providerId) => {
    const alias = getProviderAlias(providerId);
    const providerInfo = allProviders[providerId] || { name: providerId, color: "#666" };
    const isCustomProvider = isOpenAICompatibleProvider(providerId) || isAnthropicCompatibleProvider(providerId);

    if (kindFilter && PROVIDER_AS_MODEL_KINDS.has(kindFilter)) {
      groups[providerId] = {
        name: providerInfo.name,
        alias,
        color: providerInfo.color,
        models: [{ id: providerId, name: providerInfo.name, value: providerId }],
      };
      return;
    }

    if (providerInfo.passthroughModels) {
      const aliasModels = Object.entries(modelAliases).reduce((acc, [aliasName, fullModel]) => {
        if (fullModel.startsWith(`${alias}/`)) {
          acc.push({ id: fullModel.replace(`${alias}/`, ""), name: aliasName, value: fullModel });
        }
        return acc;
      }, []);

      const customRegisteredModels = customModels
        .filter((m) => m.providerAlias === alias)
        .map((m) => ({
          id: m.id,
          name: m.name || m.id,
          value: `${alias}/${m.id}`,
          kind: getModelKind(m),
          isCustom: true,
        }));
      let combined = aliasModels;
      if (kindFilter && TYPED_KINDS.has(kindFilter)) {
        const registeredTyped = customRegisteredModels.filter((m) => getModelKind(m) === kindFilter);
        combined = [
          ...registeredTyped,
          ...getModelsByProviderId(providerId)
            .filter((m) => getModelKind(m) === kindFilter)
            .map((m) => ({ id: m.id, name: m.name, value: `${alias}/${m.id}`, kind: getModelKind(m) }))
            .filter((m) => !registeredTyped.some((registered) => registered.value === m.value)),
        ];
        if (combined.length === 0 && ALLOW_PROVIDER_FALLBACK_KINDS.has(kindFilter)) {
          const supports = (providerInfo.serviceKinds || ["llm"]).includes(kindFilter);
          if (supports) combined = [{ id: providerId, name: providerInfo.name, value: alias }];
        }
      } else {
        const registeredLlms = customRegisteredModels.filter((m) => !getModelKind(m) || getModelKind(m) === "llm");
        const seen = new Set([...aliasModels, ...registeredLlms].map((m) => m.value));
        const hardcoded = getModelsByProviderId(providerId)
          .filter((m) => !getModelKind(m) || getModelKind(m) === "llm")
          .map((m) => ({ id: m.id, name: m.name, value: `${alias}/${m.id}`, kind: getModelKind(m) }))
          .filter((m) => !seen.has(m.value));
        combined = [
          ...registeredLlms,
          ...aliasModels.filter((m) => !registeredLlms.some((registered) => registered.value === m.value)),
          ...hardcoded,
        ];
      }

      if (combined.length > 0) {
        const matchedNode = providerNodes.find((node) => node.id === providerId);
        const displayName = matchedNode?.name || providerInfo.name;

        groups[providerId] = {
          name: displayName,
          alias: alias,
          color: providerInfo.color,
          models: combined,
        };
      }
    } else if (isCustomProvider) {
      if (kindFilter && TYPED_KINDS.has(kindFilter)) return;
      const connection = activeProviders.find((p) => p.provider === providerId);
      const matchedNode = providerNodes.find((node) => node.id === providerId);
      const displayName = matchedNode?.name || connection?.name || providerInfo.name;
      const nodePrefix = connection?.providerSpecificData?.prefix || matchedNode?.prefix || providerId;

      const nodeModels = Object.entries(modelAliases).reduce((acc, [aliasName, fullModel]) => {
        if (fullModel.startsWith(`${providerId}/`)) {
          acc.push({ id: fullModel.replace(`${providerId}/`, ""), name: aliasName, value: `${nodePrefix}/${fullModel.replace(`${providerId}/`, "")}` });
        }
        return acc;
      }, []);
      const registeredCustom = customModels
        .filter((m) => m.providerAlias === providerId)
        .map((m) => ({ id: m.id, name: m.name || m.id, value: `${nodePrefix}/${m.id}`, isCustom: true }));
      const seenNodeModels = new Set(nodeModels.map((m) => m.value));
      const mergedModels = [...nodeModels, ...registeredCustom.filter((m) => !seenNodeModels.has(m.value))];

      const modelsToShow = mergedModels.length > 0 ? mergedModels : [{
        id: `__placeholder__${providerId}`,
        name: `${nodePrefix}/model-id`,
        value: `${nodePrefix}/model-id`,
        isPlaceholder: true,
      }];

      groups[providerId] = {
        name: displayName,
        alias: nodePrefix,
        color: providerInfo.color,
        models: modelsToShow,
        isCustom: true,
        hasModels: mergedModels.length > 0,
      };
    } else {
      const hardcodedModels = providerId === "cursor" && cursorModels.length > 0 ? cursorModels : getModelsByProviderId(providerId);
      const hardcodedIds = new Set(hardcodedModels.map((m) => m.id));

      const hasHardcoded = hardcodedModels.length > 0;
      const customAliasModels = Object.entries(modelAliases).reduce((acc, [aliasName, fullModel]) => {
        if (!fullModel.startsWith(`${alias}/`)) return acc;
        const modelId = fullModel.replace(`${alias}/`, "");
        if ((hasHardcoded ? aliasName === modelId : true) && !hardcodedIds.has(modelId)) {
          acc.push({ id: modelId, name: aliasName, value: fullModel, isCustom: true });
        }
        return acc;
      }, []);

      const customAliasIds = new Set(customAliasModels.map((m) => m.id));
      const customRegisteredModels = customModels.reduce((acc, m) => {
        if (m.providerAlias === alias && !hardcodedIds.has(m.id) && !customAliasIds.has(m.id)) {
          acc.push({ id: m.id, name: m.name || m.id, value: `${alias}/${m.id}`, isCustom: true });
        }
        return acc;
      }, []);

      const merged = [
        ...hardcodedModels.map((m) => ({ id: m.id, name: m.name, value: `${alias}/${m.id}`, kind: getModelKind(m) })),
        ...customAliasModels,
        ...customRegisteredModels,
      ];
      const seen = new Set();
      let allModels = filterByKind(merged.filter((m) => {
        if (seen.has(m.value)) return false;
        seen.add(m.value);
        return true;
      }));

      if (allModels.length === 0 && kindFilter && ALLOW_PROVIDER_FALLBACK_KINDS.has(kindFilter)) {
        const supports = (providerInfo.serviceKinds || ["llm"]).includes(kindFilter);
        if (supports) {
          allModels = [{ id: providerId, name: providerInfo.name, value: alias }];
        }
      }

      if (allModels.length > 0) {
        groups[providerId] = {
          name: providerInfo.name,
          alias: alias,
          color: providerInfo.color,
          models: allModels,
        };
      }
    }
  });

  Object.entries(groups).forEach(([providerId, group]) => {
    const aliasKey = getProviderAlias(providerId);
    const disabledIds = new Set([
      ...(disabledModels[aliasKey] || []),
      ...(disabledModels[providerId] || []),
    ]);
    if (disabledIds.size === 0) return;
    group.models = group.models.filter((m) => !disabledIds.has(m.id));
    if (group.models.length === 0) delete groups[providerId];
  });

  return groups;
}

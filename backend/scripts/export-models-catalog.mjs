// Gera catálogo de modelos direto do open-sse (sem aliases "@/").
import { PROVIDER_MODELS } from '../../open-sse/config/providerModels.js';
import { getCapabilitiesForModel } from '../../open-sse/providers/capabilities.js';
import registry from '../../open-sse/providers/registry/index.js';
import { writeFile } from 'node:fs/promises';
const aliasOf = {};
for (const entry of registry) aliasOf[entry.id] = entry.uiAlias || entry.alias || null;
const models = Object.entries(PROVIDER_MODELS).flatMap(([alias, list]) => list.map(m => ({ provider: alias, model: m.id, name: m.name, upstreamModelId: m.upstreamModelId || null })));
const withCaps = models.map(m => {
  const fullModel = `${m.provider}/${m.model}`;
  const providerAlias = aliasOf[m.provider] || m.provider;
  const c = getCapabilitiesForModel(m.provider, m.model);
  // upstreamModelId só entra quando difere do id roteado (modelos renomeados no upstream).
  const entry = { provider: m.provider, model: m.model, name: m.name, fullModel, routedModel: `${providerAlias}/${m.model}`, caps: { vision: c.vision, search: c.search, reasoning: c.reasoning, contextWindow: c.contextWindow, maxOutput: c.maxOutput } };
  if (m.upstreamModelId) entry.upstreamModelId = m.upstreamModelId;
  return entry;
});
const passthrough = registry.filter(p => p.passthroughModels).map(p => p.id);
await writeFile(new URL('../internal/providers/models-catalog.json', import.meta.url), JSON.stringify({ models: withCaps, passthrough }, null, 2) + '\n');
console.log(`Exportados ${withCaps.length} modelos; passthrough: ${passthrough.length}`);

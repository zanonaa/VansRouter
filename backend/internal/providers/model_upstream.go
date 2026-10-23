package providers

import (
	"strings"
	"sync"
)

// Equivalente a open-sse/config/providerModels.js getModelUpstreamId e
// src/sse/services/model.js RESERVED_PROVIDER_PREFIXES.
var (
	modelMapsOnce    sync.Once
	upstreamModels   map[string]string
	reservedPrefixes map[string]bool
)

// Prefixos locais da origem (LOCAL_PROVIDER_ALIASES); não existem no registry
// mas também não podem ser sobrescritos por prefixo de provider-node.
var localProviderPrefixes = []string{"xmtp", "xiaomi-tokenplan"}

func stringField(value any) string {
	text, _ := value.(string)
	return text
}

func loadModelMaps() {
	upstreamModels = map[string]string{}
	aliases := AliasMap()
	aliasesOf := map[string][]string{}
	for alias, id := range aliases {
		aliasesOf[id] = append(aliasesOf[id], alias)
	}
	for _, entry := range CatalogModels().Models {
		provider, model := stringField(entry["provider"]), stringField(entry["model"])
		upstream := stringField(entry["upstreamModelId"])
		if provider == "" || model == "" || upstream == "" || upstream == model {
			continue
		}
		// A chave do catálogo é o alias (ds/..., cx/...); o gateway resolve para
		// o id interno (deepseek, codex), então registramos as duas formas.
		upstreamModels[provider+"/"+model] = upstream
		if resolved := aliases[provider]; resolved != "" {
			upstreamModels[resolved+"/"+model] = upstream
		}
		for _, alias := range aliasesOf[provider] {
			upstreamModels[alias+"/"+model] = upstream
		}
	}
	reservedPrefixes = map[string]bool{}
	for _, entry := range Catalog() {
		reservedPrefixes[entry.ID] = true
		if entry.Alias != nil && *entry.Alias != "" {
			reservedPrefixes[*entry.Alias] = true
		}
		for _, alias := range entry.Aliases {
			reservedPrefixes[alias] = true
		}
	}
	for _, prefix := range localProviderPrefixes {
		reservedPrefixes[prefix] = true
	}
}

// UpstreamModelID devolve o id que o upstream reconhece para o modelo roteado
// (ex.: ds/deepseek-v4-pro-max → deepseek-v4-pro). Sem mapeamento, devolve o
// próprio id. provider pode ser o id interno ou o alias do provider.
func UpstreamModelID(provider, model string) string {
	if model == "" {
		return model
	}
	modelMapsOnce.Do(loadModelMaps)
	if upstream := upstreamModels[provider+"/"+model]; upstream != "" {
		return upstream
	}
	// Codex: revisão usa o modelo base no upstream (CODEX_REVIEW_SUFFIX).
	if (provider == "cx" || provider == "codex") && strings.HasSuffix(model, "-review") {
		return strings.TrimSuffix(model, "-review")
	}
	return model
}

// ReservedPrefix informa se o prefixo pertence a um provider embutido; nesse
// caso um provider-node de mesmo prefixo não pode assumir o modelo.
func ReservedPrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	modelMapsOnce.Do(loadModelMaps)
	return reservedPrefixes[prefix]
}

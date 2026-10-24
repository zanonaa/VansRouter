package handlers

import (
	"context"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// providerNodeID resolve o prefixo de um provider-node para o id do node
// (openai-compatible-chat-<uuid> etc.). Prefixos de providers embutidos são
// reservados e não podem ser sobrescritos, como em src/sse/services/model.js.
func (g *Gateway) providerNodeID(ctx context.Context, prefix string) string {
	if g.DB == nil || prefix == "" || providers.ReservedPrefix(prefix) {
		return ""
	}
	// Mesma ordem da origem: openai-compatible, anthropic-compatible,
	// custom-embedding.
	for _, kind := range []string{"openai-compatible", "anthropic-compatible", "custom-embedding"} {
		nodes, err := (database.Nodes{DB: g.DB}).List(ctx, kind)
		if err != nil {
			continue
		}
		for _, node := range nodes {
			if ttsString(node["prefix"]) == prefix {
				return ttsString(node["id"])
			}
		}
	}
	return ""
}

// providerAllowed replica isProviderAllowed: a ACL aceita o id do provider, o
// alias e o prefixo do provider-node.
func (g *Gateway) providerAllowed(key map[string]any, provider, prefix string) bool {
	if gatewayPermission(key, "allowedProviders", provider) {
		return true
	}
	if prefix != "" && gatewayPermission(key, "allowedProviders", prefix) {
		return true
	}
	for alias, id := range providers.AliasMap() {
		if id == provider && gatewayPermission(key, "allowedProviders", alias) {
			return true
		}
	}
	return false
}

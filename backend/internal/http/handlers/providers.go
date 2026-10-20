package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type Providers struct {
	Connections      NodeConnections
	Nodes            NodeStore
	CreateConnection func(context.Context, map[string]any) (map[string]any, error)
	GetNode          func(context.Context, string) (map[string]any, error)
	GetPool          func(context.Context, string) (map[string]any, error)
}

func (h Providers) List(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to fetch providers"}) }
	connections, err := h.Connections.List(r.Context(), "", nil)
	if err != nil {
		fail()
		return
	}
	names := map[string]any{}
	if nodes, err := h.Nodes.List(r.Context(), ""); err == nil {
		for _, node := range nodes {
			id, _ := node["id"].(string)
			if nodeTruthy(node["name"]) && id != "" {
				names[id] = node["name"]
			}
		}
	}
	catalog := providers.Catalog()
	aliases := map[string]any{}
	public := []map[string]any{}
	for _, entry := range catalog {
		aliases[entry.ID] = entry.Alias
		public = append(public, map[string]any{"id": entry.ID, "alias": entry.Alias, "aliases": entry.Aliases, "displayName": entry.DisplayName, "noAuth": entry.NoAuth, "serviceKinds": entry.ServiceKinds})
	}
	safe := []map[string]any{}
	for _, connection := range connections {
		copy := map[string]any{}
		for key, value := range connection {
			switch key {
			case "apiKey", "accessToken", "refreshToken", "idToken":
			default:
				copy[key] = value
			}
		}
		id, _ := connection["provider"].(string)
		copy["alias"] = aliases[id]
		if strings.HasPrefix(id, "openai-compatible-") || strings.HasPrefix(id, "anthropic-compatible-") {
			name := connection["name"]
			if !nodeTruthy(name) {
				name = names[id]
			}
			if !nodeTruthy(name) {
				if specific, ok := connection["providerSpecificData"].(map[string]any); ok {
					name = specific["nodeName"]
				}
			}
			if !nodeTruthy(name) {
				name = id
			}
			copy["name"] = name
		}
		safe = append(safe, copy)
	}
	nodeReply(w, 200, map[string]any{"connections": safe, "aliasMap": providers.AliasMap(), "providers": public})
}

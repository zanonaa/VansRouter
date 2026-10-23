package handlers

import (
	"net/http"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func gatewayPermission(key map[string]any, field, value string) bool {
	if key == nil || key[field] == nil {
		return true
	}
	values, ok := key[field].([]any)
	if !ok {
		return false
	}
	for _, entry := range values {
		if entry == value {
			return true
		}
	}
	return false
}

// modelAuth preserves requireApiKey=false, missing/invalid distinction and CORS.
func (g *Gateway) modelAuth(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	settings, err := database.ReadAuthSettings(r.Context(), g.DB)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": map[string]any{"message": err.Error(), "type": "server_error"}})
		return nil, false
	}
	required := database.Defaults()["requireApiKey"] == true
	if value, exists := settings["requireApiKey"]; exists {
		required = nodeTruthy(value)
	}
	if !required {
		return nil, true
	}
	authorization := r.Header.Get("Authorization")
	token := ""
	if strings.HasPrefix(authorization, "Bearer ") {
		token = strings.TrimPrefix(authorization, "Bearer ")
	}
	if token == "" {
		token = r.Header.Get("x-api-key")
	}
	message := "Missing API key"
	if token != "" {
		key, err := database.ValidateApiKey(r.Context(), g.DB, token)
		if err != nil {
			nodeReply(w, 500, map[string]any{"error": map[string]any{"message": err.Error(), "type": "server_error"}})
			return nil, false
		}
		if key != nil {
			return key, true
		}
		message = "Invalid API key"
	}
	nodeReply(w, 401, map[string]any{"error": map[string]any{"message": message, "type": "authentication_error"}})
	return nil, false
}

// catalogModels lists the embedded models of active providers, without sending
// gateway credentials to an unrelated upstream /models endpoint.
func (g *Gateway) enabledModelProviders(r *http.Request) (map[string]bool, error) {
	active := true
	connections, err := (database.Connections{DB: g.DB}).List(r.Context(), "", &active)
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	for _, connection := range connections {
		enabled[ttsString(connection["provider"])] = true
	}
	for _, entry := range providers.Catalog() {
		if entry.NoAuth {
			enabled[entry.ID] = true
		}
	}
	return enabled, nil
}

func (g *Gateway) catalogModels(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	key, ok := g.modelAuth(w, r)
	if !ok {
		return
	}
	active := true
	connections, err := (database.Connections{DB: g.DB}).List(r.Context(), "", &active)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": map[string]any{"message": err.Error(), "type": "server_error"}})
		return
	}
	enabled := map[string]bool{}
	for _, connection := range connections {
		enabled[ttsString(connection["provider"])] = true
	}
	aliases := map[string]string{}
	for _, entry := range providers.Catalog() {
		alias := entry.ID
		if entry.Alias != nil && *entry.Alias != "" {
			alias = *entry.Alias
		}
		aliases[entry.ID] = alias
		if entry.NoAuth {
			enabled[entry.ID] = true
		}
	}
	data := []map[string]any{}
	seen := map[string]bool{}
	// Combos primeiro, como na origem (src/sse/services/allowedModels.js): são
	// modelos virtuais que o cliente OpenAI-compatible precisa enxergar para
	// selecioná-los. Sem isso o /v1/models só listava o catálogo dos providers.
	combos, err := (database.Combos{DB: g.DB}).List(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": map[string]any{"message": err.Error(), "type": "server_error"}})
		return
	}
	for _, combo := range combos {
		name, kind := ttsString(combo["name"]), ttsString(combo["kind"])
		if name == "" || (kind != "" && kind != "llm") {
			continue
		}
		if !gatewayPermission(key, "allowedCombos", name) {
			continue
		}
		id := "combo/" + name
		if seen[id] {
			continue
		}
		seen[id] = true
		entry := map[string]any{"id": id, "object": "model", "owned_by": "combo"}
		if length, ok := combo["context_length"].(float64); ok {
			entry["context_length"] = length
		}
		data = append(data, entry)
	}
	for _, model := range providers.CatalogModels().Models {
		provider, id := ttsString(model["provider"]), ttsString(model["model"])
		if !enabled[provider] || id == "" || !gatewayPermission(key, "allowedProviders", provider) || !gatewayPermission(key, "allowedKinds", "llm") {
			continue
		}
		alias := aliases[provider]
		if alias == "" {
			alias = provider
		}
		full := alias + "/" + id
		if seen[full] {
			continue
		}
		seen[full] = true
		data = append(data, map[string]any{"id": full, "object": "model", "kind": "llm", "owned_by": alias})
	}
	nodeReply(w, 200, map[string]any{"object": "list", "data": data})
}

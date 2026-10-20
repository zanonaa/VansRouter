package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func safeConnection(connection map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range connection {
		switch key {
		case "apiKey", "accessToken", "refreshToken", "idToken":
		default:
			result[key] = value
		}
	}
	return result
}
func (h Providers) Get(w http.ResponseWriter, r *http.Request) {
	connection, err := h.GetConnection(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch connection"})
		return
	}
	if connection == nil {
		nodeReply(w, 404, map[string]string{"error": "Connection not found"})
		return
	}
	nodeReply(w, 200, map[string]any{"connection": safeConnection(connection)})
}
func (h Providers) Delete(w http.ResponseWriter, r *http.Request) {
	deleted, err := h.DeleteConnection(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to delete connection"})
		return
	}
	if !deleted {
		nodeReply(w, 404, map[string]string{"error": "Connection not found"})
		return
	}
	nodeReply(w, 200, map[string]string{"message": "Connection deleted successfully"})
}
func (h Providers) Update(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to update connection"}) }
	reject := func(message string) { nodeReply(w, 400, map[string]string{"error": message}) }
	var body map[string]any
	decoder := json.NewDecoder(r.Body)
	if decoder.Decode(&body) != nil || body == nil {
		fail()
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		fail()
		return
	}
	id := chi.URLParam(r, "id")
	existing, err := h.GetConnection(r.Context(), id)
	if err != nil {
		fail()
		return
	}
	if existing == nil {
		nodeReply(w, 404, map[string]string{"error": "Connection not found"})
		return
	}
	str := func(key string) string { s, _ := body[key].(string); return strings.TrimSpace(s) }
	hasProxy := false
	for _, key := range []string{"connectionProxyEnabled", "connectionProxyUrl", "connectionNoProxy"} {
		if _, exists := body[key]; exists {
			hasProxy = true
		}
	}
	if hasProxy && body["connectionProxyEnabled"] == true && str("connectionProxyUrl") == "" {
		reject("Connection proxy URL is required when connection proxy is enabled")
		return
	}
	_, hasMany := body["proxyPoolIds"]
	_, hasSingle := body["proxyPoolId"]
	hasPool := hasMany || hasSingle
	poolID := ""
	poolIDs := []string{}
	strategy := "none"
	if hasMany {
		if raw, ok := body["proxyPoolIds"].([]any); ok {
			seen := map[string]bool{}
			for _, value := range raw {
				valueString := jsPoolString(value)
				if valueString != "" && !seen[valueString] {
					seen[valueString] = true
					poolIDs = append(poolIDs, valueString)
				}
			}
		}
		if value, ok := body["proxyRotationStrategy"].(string); ok {
			strategy = strings.ToLower(strings.TrimSpace(value))
		}
		switch strategy {
		case "none", "fill-first", "round-robin", "random", "smart":
		default:
			reject("Invalid proxy rotation strategy")
			return
		}
	} else if hasSingle {
		value := body["proxyPoolId"]
		if value != nil && value != "" && value != "__none__" {
			poolID = jsPoolString(value)
		}
	}
	check := poolIDs
	if poolID != "" {
		check = []string{poolID}
	}
	for _, value := range check {
		pool, err := h.GetPool(r.Context(), value)
		if err != nil {
			fail()
			return
		}
		if pool == nil {
			reject("Proxy pool not found")
			return
		}
	}
	patch := map[string]any{}
	for _, key := range []string{"name", "priority", "globalPriority", "defaultModel", "isActive", "testStatus", "lastError", "lastErrorAt", "projectId", "isProjectIdManual"} {
		if value, exists := body[key]; exists {
			patch[key] = value
		}
	}
	if nodeTruthy(body["apiKey"]) && existing["authType"] == "apikey" {
		patch["apiKey"] = body["apiKey"]
	}
	_, hadSpecific := existing["providerSpecificData"]
	_, hasSpecific := body["providerSpecificData"]
	if hadSpecific || hasSpecific || hasProxy || hasPool {
		merged := map[string]any{}
		for _, source := range []any{existing["providerSpecificData"], body["providerSpecificData"]} {
			for key, value := range jsSpread(source) {
				merged[key] = value
			}
		}
		if hasProxy {
			merged["connectionProxyEnabled"] = body["connectionProxyEnabled"] == true
			merged["connectionProxyUrl"] = str("connectionProxyUrl")
			merged["connectionNoProxy"] = str("connectionNoProxy")
		}
		if hasPool {
			delete(merged, "proxyPoolId")
			delete(merged, "proxyPoolIds")
			delete(merged, "proxyRotationStrategy")
			if poolID != "" {
				merged["proxyPoolId"] = poolID
			}
			if len(poolIDs) > 0 {
				merged["proxyPoolIds"] = poolIDs
				merged["proxyRotationStrategy"] = strategy
			}
		}
		patch["providerSpecificData"] = merged
	}
	updated, err := h.UpdateConnection(r.Context(), id, patch)
	if err != nil {
		fail()
		return
	}
	nodeReply(w, 200, map[string]any{"connection": safeConnection(updated)})
}

// IDs usados pelo frontend são strings; mantém String(null)/String(bool) nos
// demais escalares JSON. Objetos/arrays ainda requerem homologação diferencial.
func jsPoolString(value any) string {
	if value == nil {
		return "null"
	}
	return strings.TrimSpace(jsString(value))
}

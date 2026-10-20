package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func (h Providers) Create(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to create provider"}) }
	reject := func(status int, message string) { nodeReply(w, status, map[string]string{"error": message}) }
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
	rawID, _ := body["provider"].(string)
	id := providers.NormalizeID(rawID)
	str := func(key string) string { s, _ := body[key].(string); return strings.TrimSpace(s) }
	enabled := body["connectionProxyEnabled"] == true
	proxyURL := str("connectionProxyUrl")
	if enabled && proxyURL == "" {
		reject(400, "Connection proxy URL is required when connection proxy is enabled")
		return
	}
	var poolID any
	if value := body["proxyPoolId"]; value != nil && value != "" && value != "__none__" {
		// String(value).trim() para os valores escalares usados pelo frontend.
		valueString := strings.TrimSpace(fmt.Sprint(value))
		if valueString != "" {
			pool, err := h.GetPool(r.Context(), valueString)
			if err != nil {
				fail()
				return
			}
			if pool == nil {
				reject(400, "Proxy pool not found")
				return
			}
			poolID = valueString
		}
	}
	var entry *providers.Entry
	for _, candidate := range providers.Catalog() {
		if candidate.ID == id {
			copy := candidate
			entry = &copy
			break
		}
	}
	compatible := ""
	for _, kind := range []string{"openai-compatible", "anthropic-compatible", "custom-embedding"} {
		if strings.HasPrefix(id, kind+"-") {
			compatible = kind
			break
		}
	}
	if id == "" || (compatible == "" && (entry == nil || !entry.AcceptsAPIKey())) {
		reject(400, "Invalid provider")
		return
	}
	cookie := entry != nil && entry.Category == "webCookie"
	if !nodeTruthy(body["apiKey"]) && id != "ollama-local" {
		label := "API Key"
		if cookie {
			label = "Cookie value"
		}
		reject(400, label+" is required")
		return
	}
	name := body["name"]
	if !nodeTruthy(name) {
		name = body["displayName"]
	}
	if !nodeTruthy(name) && entry != nil {
		name = entry.Name
	}
	if !nodeTruthy(name) {
		reject(400, "Name is required")
		return
	}
	specific := map[string]any{}
	if original, ok := body["providerSpecificData"].(map[string]any); ok {
		for key, value := range original {
			specific[key] = value
		}
	}
	if id == "ollama-local" {
		var base any
		for _, value := range []any{specific["baseUrl"], body["baseUrl"], body["baseURL"], body["ollamaHostUrl"]} {
			if nodeTruthy(value) {
				base = value
				break
			}
		}
		if base != nil {
			value, ok := base.(string)
			if !ok {
				fail()
				return
			}
			if value = strings.TrimSpace(value); value != "" {
				specific["baseUrl"] = value
			}
		}
	}
	if compatible != "" {
		node, err := h.GetNode(r.Context(), id)
		if err != nil {
			fail()
			return
		}
		if node == nil {
			label := map[string]string{"openai-compatible": "OpenAI Compatible", "anthropic-compatible": "Anthropic Compatible", "custom-embedding": "Custom Embedding"}[compatible]
			reject(404, label+" node not found")
			return
		}
		for _, key := range []string{"prefix", "baseUrl"} {
			if value, ok := node[key]; ok {
				specific[key] = value
			} else {
				delete(specific, key)
			}
		}
		if value, ok := node["name"]; ok {
			specific["nodeName"] = value
		} else {
			delete(specific, "nodeName")
		}
		if compatible == "openai-compatible" {
			if value, ok := node["apiType"]; ok {
				specific["apiType"] = value
			} else {
				delete(specific, "apiType")
			}
		}
	}
	specific["connectionProxyEnabled"] = enabled
	specific["connectionProxyUrl"] = proxyURL
	specific["connectionNoProxy"] = str("connectionNoProxy")
	if poolID != nil {
		specific["proxyPoolId"] = poolID
	}
	fallback := func(key string, value any) any {
		if nodeTruthy(body[key]) {
			return body[key]
		}
		return value
	}
	authType := "apikey"
	if cookie {
		authType = "cookie"
	}
	data := map[string]any{"provider": id, "authType": authType, "name": name, "apiKey": fallback("apiKey", ""), "priority": fallback("priority", float64(1)), "globalPriority": fallback("globalPriority", nil), "defaultModel": fallback("defaultModel", nil), "providerSpecificData": specific, "isActive": true, "testStatus": fallback("testStatus", "unknown")}
	conn, err := h.CreateConnection(r.Context(), data)
	if err != nil {
		fail()
		return
	}
	result := map[string]any{}
	for key, value := range conn {
		if key != "apiKey" {
			result[key] = value
		}
	}
	nodeReply(w, 201, map[string]any{"connection": result})
}

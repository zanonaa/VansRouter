package handlers

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func (h Providers) Bulk(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to create provider connections"}) }
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
	raw, ok := body["items"].([]any)
	if !ok || len(raw) == 0 || len(raw) > 500 {
		reject(400, "items must contain 1-500 connections")
		return
	}
	items := []map[string]any{}
	for _, value := range raw {
		if value == nil {
			fail()
			return
		}
		input, _ := value.(map[string]any)
		id, _ := input["provider"].(string)
		id = providers.NormalizeID(id)
		apiKey, _ := input["apiKey"].(string)
		name, _ := input["name"].(string)
		priority := float64(1)
		if p, ok := input["priority"].(float64); ok && math.Trunc(p) == p {
			priority = p
		}
		item := map[string]any{"provider": id, "apiKey": strings.TrimSpace(apiKey), "name": strings.TrimSpace(name), "priority": priority, "testStatus": "unknown"}
		if specific, exists := input["providerSpecificData"]; exists {
			item["providerSpecificData"] = specific
		}
		items = append(items, item)
	}
	// Bulk intentionally does NOT accept OAuth dual-auth providers via authModes.
	for _, item := range items {
		id := item["provider"].(string)
		valid := false
		for _, entry := range providers.Catalog() {
			if entry.ID == id && (entry.Category == "apikey" || entry.Category == "freeTier" || entry.Category == "webCookie") {
				valid = true
				break
			}
		}
		for _, prefix := range []string{"openai-compatible-", "anthropic-compatible-", "custom-embedding-"} {
			if strings.HasPrefix(id, prefix) {
				valid = true
			}
		}
		if !valid || item["apiKey"] == "" || item["name"] == "" {
			reject(400, "Every item requires a valid API-key provider, name, and apiKey")
			return
		}
	}
	names := map[string]bool{}
	nodes := map[string]map[string]any{}
	for _, item := range items {
		id := item["provider"].(string)
		key := id + ":" + item["name"].(string)
		if names[key] {
			reject(409, "Duplicate provider/name in batch")
			return
		}
		names[key] = true
		kind := ""
		for _, prefix := range []string{"openai-compatible", "anthropic-compatible", "custom-embedding"} {
			if strings.HasPrefix(id, prefix+"-") {
				kind = prefix
				break
			}
		}
		if kind == "" {
			continue
		}
		node, cached := nodes[id]
		if !cached {
			var err error
			node, err = h.GetNode(r.Context(), id)
			if err != nil {
				fail()
				return
			}
			nodes[id] = node
		}
		if node == nil {
			label := map[string]string{"openai-compatible": "OpenAI Compatible", "anthropic-compatible": "Anthropic Compatible", "custom-embedding": "Custom Embedding"}[kind]
			reject(404, label+" node not found")
			return
		}
		specific := jsSpread(item["providerSpecificData"])
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
		if kind == "openai-compatible" {
			if value, ok := node["apiType"]; ok {
				specific["apiType"] = value
			} else {
				delete(specific, "apiType")
			}
		}
		item["providerSpecificData"] = specific
	}
	results, err := h.CreateBulk(r.Context(), items)
	if err != nil {
		fail()
		return
	}
	nodeReply(w, 201, map[string]any{"results": results})
}

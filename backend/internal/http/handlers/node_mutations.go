package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (h Nodes) Update(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to update provider node"}) }
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
	node, err := h.Mutations.Get(r.Context(), id)
	if err != nil {
		fail()
		return
	}
	if node == nil {
		nodeReply(w, 404, map[string]string{"error": "Provider node not found"})
		return
	}
	values := map[string]string{}
	for _, field := range []struct{ key, message string }{{"name", "Name is required"}, {"prefix", "Prefix is required"}} {
		raw := body[field.key]
		if raw != nil {
			value, ok := raw.(string)
			if !ok {
				fail()
				return
			}
			values[field.key] = strings.TrimSpace(value)
		}
		if values[field.key] == "" {
			nodeReply(w, 400, map[string]string{"error": field.message})
			return
		}
	}
	apiType, _ := body["apiType"].(string)
	if node["type"] == "openai-compatible" && apiType != "chat" && apiType != "responses" {
		nodeReply(w, 400, map[string]string{"error": "Invalid OpenAI compatible API type"})
		return
	}
	raw := body["baseUrl"]
	base := ""
	if raw != nil {
		value, ok := raw.(string)
		if !ok {
			fail()
			return
		}
		base = strings.TrimSpace(value)
	}
	if base == "" {
		nodeReply(w, 400, map[string]string{"error": "Base URL is required"})
		return
	}
	if node["type"] == "anthropic-compatible" {
		base = strings.TrimSuffix(base, "/")
		base = strings.TrimSuffix(base, "/messages")
		if !strings.HasSuffix(base, "/v1") {
			base += "/v1"
		}
	}
	if node["type"] == "custom-embedding" {
		base = strings.TrimSuffix(base, "/")
		base = strings.TrimSuffix(base, "/embeddings")
	}
	updates := map[string]any{"name": values["name"], "prefix": values["prefix"], "baseUrl": base}
	if node["type"] == "openai-compatible" {
		updates["apiType"] = apiType
	}
	// A origem também pode ter alterações parciais: nó e connections são escritos
	// em transações separadas. Não simular atomicidade global nem sucesso em falha.
	updated, err := h.Mutations.Update(r.Context(), id, updates)
	if err != nil {
		fail()
		return
	}
	connections, err := h.Connections.List(r.Context(), id, nil)
	if err != nil {
		fail()
		return
	}
	for _, connection := range connections {
		specific := map[string]any{}
		if old, ok := connection["providerSpecificData"].(map[string]any); ok {
			for k, v := range old {
				specific[k] = v
			}
		}
		specific["prefix"] = values["prefix"]
		specific["baseUrl"] = base
		specific["nodeName"] = updated["name"]
		if node["type"] == "openai-compatible" {
			specific["apiType"] = apiType
		} else {
			delete(specific, "apiType")
		}
		connectionID, _ := connection["id"].(string)
		if _, err = h.Connections.Update(r.Context(), connectionID, map[string]any{"providerSpecificData": specific}); err != nil {
			fail()
			return
		}
	}
	nodeReply(w, 200, map[string]any{"node": updated})
}
func (h Nodes) Delete(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to delete provider node"}) }
	id := chi.URLParam(r, "id")
	node, err := h.Mutations.Get(r.Context(), id)
	if err != nil {
		fail()
		return
	}
	if node == nil {
		nodeReply(w, 404, map[string]string{"error": "Provider node not found"})
		return
	}
	if _, err = h.Connections.DeleteByProvider(r.Context(), id); err != nil {
		fail()
		return
	}
	if _, err = h.Mutations.Delete(r.Context(), id); err != nil {
		fail()
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}

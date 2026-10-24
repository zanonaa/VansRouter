package handlers

import (
	"context"
	"net/http"
	"strings"
)

type AliasStore interface {
	All(context.Context, string) (map[string]any, error)
	Set(context.Context, any, any) error
	Delete(context.Context, string, string) error
	CustomList(context.Context) ([]any, error)
	AddCustom(context.Context, map[string]any) (bool, error)
	CustomBulk(context.Context, []map[string]any) (int, error)
}
type ModelAliases struct{ Store AliasStore }

func (h ModelAliases) List(w http.ResponseWriter, r *http.Request) {
	values, err := h.Store.All(r.Context(), "modelAliases")
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch aliases"})
		return
	}
	nodeReply(w, 200, map[string]any{"aliases": values})
}
func (h ModelAliases) Set(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to update alias"})
		return
	}
	if !nodeTruthy(body["model"]) || !nodeTruthy(body["alias"]) {
		nodeReply(w, 400, map[string]string{"error": "Model and alias required"})
		return
	}
	if err = h.Store.Set(r.Context(), body["alias"], body["model"]); err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to update alias"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "model": body["model"], "alias": body["alias"]})
}
func (h ModelAliases) Delete(w http.ResponseWriter, r *http.Request) {
	alias := r.URL.Query().Get("alias")
	if alias == "" {
		nodeReply(w, 400, map[string]string{"error": "Alias required"})
		return
	}
	if err := h.Store.Delete(r.Context(), "modelAliases", alias); err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to delete alias"})
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}
func (h ModelAliases) CustomList(w http.ResponseWriter, r *http.Request) {
	values, err := h.Store.CustomList(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch custom models"})
		return
	}
	nodeReply(w, 200, map[string]any{"models": values})
}
func (h ModelAliases) CustomAdd(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to add custom model"}) }
	body, err := poolInput(r)
	if err != nil {
		fail()
		return
	}
	if models, ok := body["models"].([]any); ok {
		items := []map[string]any{}
		for _, value := range models {
			item, _ := value.(map[string]any)
			items = append(items, item)
		}
		count, err := h.Store.CustomBulk(r.Context(), items)
		if err != nil {
			fail()
			return
		}
		nodeReply(w, 200, map[string]any{"success": true, "count": count})
		return
	}
	if !nodeTruthy(body["providerAlias"]) || !nodeTruthy(body["id"]) {
		nodeReply(w, 400, map[string]string{"error": "providerAlias and id required"})
		return
	}
	kind := body["type"]
	if !nodeTruthy(kind) {
		kind = "llm"
	}
	data := map[string]any{"providerAlias": body["providerAlias"], "id": body["id"], "type": kind, "name": body["name"]}
	caps := map[string]any{}
	if input, ok := body["caps"].(map[string]any); ok {
		for _, key := range []string{"vision", "reasoning"} {
			if value, ok := input[key].(bool); ok {
				caps[key] = value
			}
		}
	}
	if len(caps) > 0 {
		data["caps"] = caps
	}
	if transport, ok := body["transport"].(string); ok && kind == "stt" && strings.TrimSpace(transport) == "gemini-live" {
		data["transport"] = strings.TrimSpace(transport)
	}
	added, err := h.Store.AddCustom(r.Context(), data)
	if err != nil {
		fail()
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "added": added})
}
func (h ModelAliases) CustomDelete(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	provider, id := query.Get("providerAlias"), query.Get("id")
	kind := query.Get("type")
	if kind == "" {
		kind = "llm"
	}
	if provider == "" || id == "" {
		nodeReply(w, 400, map[string]string{"error": "providerAlias and id required"})
		return
	}
	if err := h.Store.Delete(r.Context(), "customModels", provider+"|"+id+"|"+kind); err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to delete custom model"})
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}

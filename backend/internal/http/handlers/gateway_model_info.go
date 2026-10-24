package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func (g *Gateway) ModelInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	id, kind := r.URL.Query().Get("id"), r.URL.Query().Get("kind")
	if id == "" {
		nodeReply(w, 400, map[string]any{"error": map[string]any{"message": "Missing required query param: id (e.g. ?id=openai/dall-e-3)", "type": "invalid_request_error"}})
		return
	}
	parts := strings.SplitN(id, "/", 2)
	if len(parts) == 2 {
		for _, entry := range providers.ModelInfo() {
			matches := entry["id"] == id || entry["provider"] == parts[0] && strings.TrimPrefix(ttsString(entry["id"]), ttsString(entry["alias"])+"/") == parts[1]
			if !matches || kind != "" && entry["kind"] != kind {
				continue
			}
			out := map[string]any{}
			for key, value := range entry {
				if key != "provider" && key != "alias" {
					out[key] = value
				}
			}
			out["id"] = id
			out["owned_by"] = parts[0]
			nodeReply(w, 200, out)
			return
		}
	}
	nodeReply(w, 404, map[string]any{"error": map[string]any{"message": "Model not found: " + id, "type": "not_found"}})
}

func (g *Gateway) ModelsByKind(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	slug := chi.URLParam(r, "kind")
	kinds := map[string][]string{"image": {"image"}, "tts": {"tts"}, "stt": {"stt"}, "embedding": {"embedding"}, "video": {"video"}, "image-to-text": {"imageToText"}, "web": {"webSearch", "webFetch"}}[slug]
	if kinds == nil {
		nodeReply(w, 404, map[string]any{"error": map[string]any{"message": "Unknown model kind: " + slug + ". Supported: image, tts, stt, embedding, video, image-to-text, web", "type": "invalid_request_error"}})
		return
	}
	key, ok := g.modelAuth(w, r)
	if !ok {
		return
	}
	enabled, err := g.enabledModelProviders(r)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": map[string]any{"message": err.Error(), "type": "server_error"}})
		return
	}
	data := []map[string]any{}
	combos, err := (database.Combos{DB: g.DB}).List(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": map[string]any{"message": err.Error(), "type": "server_error"}})
		return
	}
	for _, combo := range combos {
		name, kind := ttsString(combo["name"]), ttsString(combo["kind"])
		// Combo sem kind é LLM e só aparece em /models; /models/{kind} cobre mídia.
		if name == "" || kind == "" || !gatewayPermission(key, "allowedCombos", name) {
			continue
		}
		selected := false
		for _, candidate := range kinds {
			if candidate == kind {
				selected = true
				break
			}
		}
		if !selected {
			continue
		}
		data = append(data, map[string]any{"id": "combo/" + name, "object": "model", "kind": kind, "owned_by": "combo"})
	}
	for _, entry := range providers.ModelInfo() {
		provider, kind := ttsString(entry["provider"]), ttsString(entry["kind"])
		selected := false
		for _, candidate := range kinds {
			if candidate == kind {
				selected = true
				break
			}
		}
		if !selected || !enabled[provider] || !gatewayPermission(key, "allowedProviders", provider) || !gatewayPermission(key, "allowedKinds", kind) {
			continue
		}
		data = append(data, map[string]any{"id": entry["id"], "object": "model", "kind": kind, "owned_by": entry["owned_by"]})
	}
	nodeReply(w, 200, map[string]any{"object": "list", "data": data})
}

func GatewayModelsOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
}

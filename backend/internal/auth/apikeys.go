package auth

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// APIKeysGet reproduz GET /api/keys.
func (h *HTTP) APIKeysGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	keys, err := h.ListAPIKeys(r.Context())
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to fetch keys"})
		return
	}
	if keys == nil {
		keys = []map[string]any{}
	}
	reply(w, 200, map[string]any{"keys": keys})
}

// APIKeysPost reproduz POST /api/keys.
func (h *HTTP) APIKeysPost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		reply(w, 500, map[string]string{"error": "Failed to create key"})
		return
	}
	name, _ := body["name"].(string)
	name = strings.TrimSpace(name)
	if name == "" {
		reply(w, 400, map[string]string{"error": "Name is required"})
		return
	}
	machineId := h.MachineID()
	if machineId == "" {
		reply(w, 500, map[string]string{"error": "Failed to create key"})
		return
	}
	key, err := h.CreateAPIKey(r.Context(), name, machineId)
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to create key"})
		return
	}
	reply(w, 201, map[string]any{
		"key":       key["key"],
		"name":      key["name"],
		"id":        key["id"],
		"machineId": key["machineId"],
	})
}

// APIKeyGet reproduz GET /api/keys/{id}.
func (h *HTTP) APIKeyGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id := chi.URLParam(r, "id")
	key, err := h.GetAPIKey(r.Context(), id)
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to fetch key"})
		return
	}
	if key == nil {
		reply(w, 404, map[string]string{"error": "Key not found"})
		return
	}
	reply(w, 200, map[string]any{"key": key})
}

// APIKeyPut reproduz PUT /api/keys/{id}.
func (h *HTTP) APIKeyPut(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id := chi.URLParam(r, "id")
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		reply(w, 500, map[string]string{"error": "Failed to update key"})
		return
	}
	existing, err := h.GetAPIKey(r.Context(), id)
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to update key"})
		return
	}
	if existing == nil {
		reply(w, 404, map[string]string{"error": "Key not found"})
		return
	}
	updateData := map[string]any{}
	if v, ok := body["isActive"]; ok {
		updateData["isActive"] = v
	}
	if v, ok := body["name"]; ok {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			updateData["name"] = strings.TrimSpace(s)
		}
	}
	for _, field := range []string{"allowedProviders", "allowedCombos", "allowedKinds"} {
		if v, ok := body[field]; ok {
			if v == nil {
				updateData[field] = nil
			} else if arr, ok := v.([]any); ok {
				updateData[field] = arr
			} else {
				updateData[field] = nil
			}
		}
	}
	updated, err := h.UpdateAPIKey(r.Context(), id, updateData)
	if err != nil || updated == nil {
		reply(w, 500, map[string]string{"error": "Failed to update key"})
		return
	}
	reply(w, 200, map[string]any{"key": updated})
}

// APIKeyDelete reproduz DELETE /api/keys/{id}.
func (h *HTTP) APIKeyDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	id := chi.URLParam(r, "id")
	deleted, err := h.DeleteAPIKey(r.Context(), id)
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to delete key"})
		return
	}
	if !deleted {
		reply(w, 404, map[string]string{"error": "Key not found"})
		return
	}
	reply(w, 200, map[string]string{"message": "Key deleted successfully"})
}

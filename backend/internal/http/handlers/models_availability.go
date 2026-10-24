package handlers

// Porta de src/app/api/models/availability/route.js (GET/POST).
import (
	"net/http"
	"strings"
	"time"
)

const modelLockPrefix = "modelLock_"

// ModelAvailability expõe GET/POST /api/models/availability.
type ModelAvailability struct {
	Connections NodeConnections
}

// activeModelLocks: modelLock_* com data futura.
func activeModelLocks(connection map[string]any) []map[string]any {
	now := time.Now().UnixMilli()
	var locks []map[string]any
	for key, value := range connection {
		if !strings.HasPrefix(key, modelLockPrefix) {
			continue
		}
		until := jsString(value)
		parsed, err := time.Parse(time.RFC3339, until)
		if err != nil || parsed.UnixMilli() <= now {
			continue
		}
		model := strings.TrimPrefix(key, modelLockPrefix)
		if model == "" {
			model = "__all"
		}
		locks = append(locks, map[string]any{
			"key":    key,
			"model":  model,
			"until":  until,
			"active": true,
		})
	}
	return locks
}

func availabilityConnectionName(connection map[string]any) any {
	for _, key := range []string{"name", "email", "id"} {
		if value := ttsString(connection[key]); value != "" {
			return value
		}
	}
	return nil
}

func availabilityLastError(connection map[string]any) any {
	if value := ttsString(connection["lastError"]); value != "" {
		return value
	}
	return nil
}

func (m *ModelAvailability) Get(w http.ResponseWriter, r *http.Request) {
	connections, err := m.Connections.List(r.Context(), "", nil)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": "Failed to fetch model availability"})
		return
	}
	models := []map[string]any{}
	for _, connection := range connections {
		locks := activeModelLocks(connection)
		for _, lock := range locks {
			models = append(models, map[string]any{
				"provider":       jsString(connection["provider"]),
				"model":          lock["model"],
				"status":         "cooldown",
				"until":          lock["until"],
				"connectionId":   jsString(connection["id"]),
				"connectionName": availabilityConnectionName(connection),
				"lastError":      availabilityLastError(connection),
			})
		}
		if len(locks) == 0 && jsString(connection["testStatus"]) == "unavailable" {
			models = append(models, map[string]any{
				"provider":       jsString(connection["provider"]),
				"model":          "__all",
				"status":         "unavailable",
				"connectionId":   jsString(connection["id"]),
				"connectionName": availabilityConnectionName(connection),
				"lastError":      availabilityLastError(connection),
			})
		}
	}
	nodeReply(w, 200, map[string]any{"models": models, "unavailableCount": len(models)})
}

func (m *ModelAvailability) Post(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		nodeReply(w, 400, map[string]any{"error": "Invalid request"})
		return
	}
	action := jsString(body["action"])
	provider := jsString(body["provider"])
	model := jsString(body["model"])
	if action != "clearCooldown" || !nodeTruthy(provider) || !nodeTruthy(model) {
		nodeReply(w, 400, map[string]any{"error": "Invalid request"})
		return
	}
	connections, err := m.Connections.List(r.Context(), provider, nil)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": "Failed to clear cooldown"})
		return
	}
	lockKey := modelLockPrefix + model
	for _, connection := range connections {
		if !nodeTruthy(connection[lockKey]) {
			continue
		}
		patch := map[string]any{lockKey: nil}
		if jsString(connection["testStatus"]) == "unavailable" {
			patch["testStatus"] = "active"
			patch["lastError"] = nil
			patch["lastErrorAt"] = nil
			patch["backoffLevel"] = 0
		}
		if _, updateErr := m.Connections.Update(r.Context(), jsString(connection["id"]), patch); updateErr != nil {
			nodeReply(w, 500, map[string]any{"error": "Failed to clear cooldown"})
			return
		}
	}
	nodeReply(w, 200, map[string]any{"ok": true})
}

package handlers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func providerAuthGroup(connection map[string]any) string {
	provider := ttsString(connection["provider"])
	free := false
	fallback := "apikey"
	for _, entry := range providers.Catalog() {
		if entry.ID == provider {
			free = entry.Category == "free"
			if entry.Category == "oauth" {
				fallback = "oauth"
			}
			break
		}
	}
	if auth := ttsString(connection["authType"]); auth != "" {
		if auth == "oauth" && free {
			return "free"
		}
		return auth
	}
	if free {
		return "free"
	}
	if strings.HasPrefix(provider, "openai-compatible-") || strings.HasPrefix(provider, "anthropic-compatible-") {
		return "compatible"
	}
	return fallback
}
func (p *ConnectionProbe) Batch(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]any{"error": "Batch test failed"}) }
	body, err := poolInput(r)
	if err != nil {
		fail()
		return
	}
	mode := ttsString(body["mode"])
	provider := ttsString(body["providerId"])
	if mode == "" {
		nodeReply(w, 400, map[string]any{"error": "mode is required"})
		return
	}
	switch mode {
	case "provider":
		if provider == "" {
			nodeReply(w, 400, map[string]any{"error": "Invalid mode. Use: provider, oauth, free, apikey, compatible, all"})
			return
		}
	case "oauth", "free", "apikey", "compatible", "all":
	default:
		nodeReply(w, 400, map[string]any{"error": "Invalid mode. Use: provider, oauth, free, apikey, compatible, all"})
		return
	}
	if p.ListConnections == nil {
		fail()
		return
	}
	active := true
	connections, err := p.ListConnections(r.Context(), "", &active)
	if err != nil {
		fail()
		return
	}
	selected := []map[string]any{}
	for _, connection := range connections {
		id := ttsString(connection["provider"])
		matches := mode == "all" || mode == "provider" && id == provider || mode == "compatible" && (strings.HasPrefix(id, "openai-compatible-") || strings.HasPrefix(id, "anthropic-compatible-")) || mode != "provider" && mode != "compatible" && mode == providerAuthGroup(connection)
		if matches {
			selected = append(selected, connection)
		}
	}
	results := make([]map[string]any, len(selected))
	var wg sync.WaitGroup
	for index, connection := range selected {
		wg.Add(1)
		go func(index int, connection map[string]any) {
			defer wg.Done()
			result, err := p.Test(r.Context(), ttsString(connection["id"]))
			if err != nil {
				result = map[string]any{"valid": false, "latencyMs": 0, "error": err.Error(), "diagnosis": map[string]any{"type": "network_error", "source": "local", "code": nil, "message": err.Error()}}
			}
			entry := map[string]any{"provider": connection["provider"], "connectionId": connection["id"], "connectionName": firstNonEmpty(connection["name"], connection["email"], connection["provider"]), "authType": firstNonEmpty(connection["authType"], providerAuthGroup(connection)), "valid": result["valid"], "latencyMs": firstNonEmpty(result["latencyMs"], 0), "error": result["error"], "diagnosis": result["diagnosis"], "statusCode": result["statusCode"], "testedAt": firstNonEmpty(result["testedAt"], time.Now().UTC().Format(time.RFC3339Nano))}
			results[index] = entry
		}(index, connection)
	}
	wg.Wait()
	passed := 0
	for _, result := range results {
		if result["valid"] == true {
			passed++
		}
	}
	nodeReply(w, 200, map[string]any{"mode": mode, "providerId": nilOrNilStringValue(provider), "results": results, "summary": map[string]any{"total": len(results), "passed": passed, "failed": len(results) - passed}, "testedAt": time.Now().UTC().Format("2006-01-02T15:04:05.000Z")})
}

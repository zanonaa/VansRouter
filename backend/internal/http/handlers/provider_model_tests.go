package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type ProviderModelTests struct {
	Connections usageConnectionStore
	Nodes       NodeMutations
	Ping        *ModelPing
}

func (p *ProviderModelTests) Post(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]any{"error": "Test failed"}) }
	id := chi.URLParam(r, "id")
	connection, err := p.Connections.Get(r.Context(), id)
	if err != nil {
		fail()
		return
	}
	if connection == nil {
		nodeReply(w, 404, map[string]any{"error": "Connection not found"})
		return
	}
	provider := ttsString(connection["provider"])
	alias := provider
	for _, entry := range providers.Catalog() {
		if entry.ID == provider && entry.Alias != nil {
			alias = *entry.Alias
			break
		}
	}
	compatible := strings.HasPrefix(provider, "openai-compatible-") || strings.HasPrefix(provider, "anthropic-compatible-")
	if compatible && p.Nodes != nil {
		if node, err := p.Nodes.Get(r.Context(), provider); err == nil && node != nil && nodeTruthy(node["prefix"]) {
			alias = ttsString(node["prefix"])
		}
	}
	models := []map[string]any{}
	for _, entry := range providers.ModelInfo() {
		if entry["provider"] == provider {
			models = append(models, map[string]any{"id": strings.TrimPrefix(ttsString(entry["id"]), ttsString(entry["alias"])+"/"), "name": entry["name"], "kind": entry["kind"]})
		}
	}
	if len(models) == 0 && compatible {
		models = p.liveModels(r.Context(), id)
	}
	if len(models) == 0 {
		nodeReply(w, 400, map[string]any{"error": "No models configured for this provider"})
		return
	}
	if p.Ping == nil {
		fail()
		return
	}
	results := make([]map[string]any, len(models))
	ping := func(index int) {
		model := models[index]
		kind := ttsString(firstNonEmpty(model["kind"], model["type"], "llm"))
		result := p.Ping.Ping(r.Context(), alias+"/"+ttsString(model["id"]), kind)
		result["modelId"] = model["id"]
		result["name"] = firstNonEmpty(model["name"], model["id"])
		results[index] = result
	}
	// Warm-up must finish before fan-out: token rotation must not race.
	ping(0)
	var wg sync.WaitGroup
	for index := 1; index < len(models); index++ {
		wg.Add(1)
		go func(index int) { defer wg.Done(); ping(index) }(index)
	}
	wg.Wait()
	nodeReply(w, 200, map[string]any{"provider": provider, "connectionId": id, "results": results})
}
func (p *ProviderModelTests) liveModels(ctx context.Context, id string) []map[string]any {
	if p.Ping == nil {
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(p.Ping.BaseURL, "/")+"/api/providers/"+id+"/models", nil)
	if err != nil {
		return nil
	}
	if p.Ping.Headers != nil {
		headers, err := p.Ping.Headers(ctx)
		if err != nil {
			return nil
		}
		request.Header = headers
	}
	client := p.Ping.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil
	}
	var payload struct {
		Models []map[string]any `json:"models"`
	}
	if json.NewDecoder(response.Body).Decode(&payload) != nil {
		return nil
	}
	out := []map[string]any{}
	for _, model := range payload.Models {
		out = append(out, map[string]any{"id": firstNonEmpty(model["id"], model["name"]), "name": firstNonEmpty(model["name"], model["id"])})
	}
	return out
}

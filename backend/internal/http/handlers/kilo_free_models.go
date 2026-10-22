package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// KiloFreeModels mantém o cache de uma hora e fallback stale da origem.
type KiloFreeModels struct {
	Client   *http.Client
	mu       sync.Mutex
	models   []map[string]any
	cachedAt time.Time
}

func (k *KiloFreeModels) Get(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	k.mu.Lock()
	cached, stamp := k.models, k.cachedAt
	k.mu.Unlock()
	if cached != nil && now.Sub(stamp) < time.Hour {
		nodeReply(w, 200, map[string]any{"models": cached, "cached": true})
		return
	}
	models, err := k.fetch(r.Context())
	if err != nil {
		if cached != nil {
			nodeReply(w, 200, map[string]any{"models": cached, "cached": true, "warning": err.Error()})
			return
		}
		nodeReply(w, 502, map[string]any{"models": []any{}, "error": "Failed to fetch Kilo models: " + err.Error()})
		return
	}
	k.mu.Lock()
	k.models = models
	k.cachedAt = now
	k.mu.Unlock()
	nodeReply(w, 200, map[string]any{"models": models, "cached": false})
}

func (k *KiloFreeModels) fetch(ctx context.Context) ([]map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", "https://api.kilo.ai/api/gateway/models", nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Accept", "application/json")
	client := k.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("Kilo API returned %d", response.StatusCode)
	}
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return nil, err
	}
	models := []map[string]any{}
	for _, model := range payload.Data {
		if model["isFree"] == true {
			entry := map[string]any{"isFree": true, "context_length": 0}
			for _, key := range []string{"id", "name"} {
				if value, exists := model[key]; exists {
					entry[key] = value
				}
			}
			if nodeTruthy(model["context_length"]) {
				entry["context_length"] = model["context_length"]
			}
			models = append(models, entry)
		}
	}
	return models, nil
}

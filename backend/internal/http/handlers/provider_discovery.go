package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type ProviderDiscovery struct {
	Connections usageConnectionStore
	Client      *http.Client
}

func (d *ProviderDiscovery) Get(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]any{"error": "Failed to fetch models"}) }
	connection, err := d.Connections.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		fail()
		return
	}
	if connection == nil {
		nodeReply(w, 404, map[string]any{"error": "Connection not found"})
		return
	}
	provider := ttsString(connection["provider"])
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	config := providers.Discovery(provider)
	headers := map[string]string{"Content-Type": "application/json"}
	target := ""
	public := false
	authHeader, authPrefix := "Authorization", "Bearer "
	if config != nil {
		target = config.URL
		public = config.Public
		authHeader = config.AuthHeader
		authPrefix = config.AuthPrefix
	}
	if strings.HasPrefix(provider, "openai-compatible-") {
		base := ttsString(metadata["baseUrl"])
		if base == "" {
			nodeReply(w, 400, map[string]any{"error": "No base URL configured for OpenAI compatible provider"})
			return
		}
		target = strings.TrimSuffix(base, "/") + "/models"
	} else if strings.HasPrefix(provider, "anthropic-compatible-") {
		base := ttsString(metadata["baseUrl"])
		if base == "" {
			nodeReply(w, 400, map[string]any{"error": "No base URL configured for Anthropic compatible provider"})
			return
		}
		base = strings.TrimSuffix(strings.TrimSuffix(base, "/"), "/messages")
		target = base + "/models"
		headers["x-api-key"] = ttsString(connection["apiKey"])
		headers["anthropic-version"] = "2023-06-01"
	}
	switch provider {
	case "claude", "anthropic":
		target = "https://api.anthropic.com/v1/models"
		authHeader = "x-api-key"
		authPrefix = ""
		headers["Anthropic-Version"] = "2023-06-01"
	case "gemini":
		target = "https://generativelanguage.googleapis.com/v1beta/models"
	case "qwen":
		target = "https://portal.qwen.ai/v1/models"
		if resource := strings.TrimSpace(ttsString(metadata["resourceUrl"])); resource != "" {
			if strings.HasPrefix(resource, "http://") || strings.HasPrefix(resource, "https://") {
				target = strings.TrimSuffix(resource, "/") + "/models"
			} else {
				target = "https://" + strings.TrimSuffix(resource, "/") + "/v1/models"
			}
		}
	}
	if target == "" {
		nodeReply(w, 400, map[string]any{"error": fmt.Sprintf("Provider %s does not support models listing", provider)})
		return
	}
	token := ttsString(firstNonEmpty(metadata["copilotToken"], connection["accessToken"], connection["apiKey"]))
	if !public && token == "" {
		nodeReply(w, 401, map[string]any{"error": "No valid token found"})
		return
	}
	if provider == "gemini" {
		target += "?key=" + url.QueryEscape(token)
	} else if token != "" && !public {
		headers[authHeader] = authPrefix + token
	}
	request, err := http.NewRequestWithContext(r.Context(), "GET", target, nil)
	if err != nil {
		fail()
		return
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		fail()
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		nodeReply(w, response.StatusCode, map[string]any{"error": fmt.Sprintf("Failed to fetch models: %d", response.StatusCode)})
		return
	}
	var payload any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		fail()
		return
	}
	models := any([]any{})
	switch data := payload.(type) {
	case []any:
		models = data
	case map[string]any:
		for _, field := range []string{"data", "models", "results"} {
			if value := data[field]; value != nil {
				models = value
				break
			}
		}
	}
	nodeReply(w, 200, map[string]any{"provider": provider, "connectionId": connection["id"], "models": models})
}

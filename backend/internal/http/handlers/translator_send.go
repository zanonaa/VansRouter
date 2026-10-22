package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// TranslatorSend executes OpenAI-format transports with actual streaming. Other
// formats remain explicit pending contracts rather than silently mis-translated.
type TranslatorSend struct {
	Connections NodeConnections
	Client      *http.Client
	GetPool     connectionPoolLookup
}

func (t *TranslatorSend) Send(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	input, err := poolInput(r)
	if err != nil {
		fail(500, err.Error())
		return
	}
	provider, model := ttsString(input["provider"]), ttsString(input["model"])
	if provider == "" || model == "" || !nodeTruthy(input["body"]) {
		fail(400, "provider, model, and body required")
		return
	}
	connections, err := t.Connections.List(r.Context(), provider, nil)
	if err != nil {
		fail(500, err.Error())
		return
	}
	var connection map[string]any
	for _, candidate := range connections {
		if candidate["isActive"] != false {
			connection = candidate
			break
		}
	}
	if connection == nil {
		fail(400, "No active connection for provider: "+provider)
		return
	}
	config := providers.OpenAITransport(provider)
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	if providers.TargetFormat(provider) == "claude" {
		config = providers.TransportFor(provider)
	}
	if providers.TargetFormat(provider) == "gemini" {
		config = &providers.Transport{BaseURL: "https://generativelanguage.googleapis.com/v1beta/models/" + url.PathEscape(model) + ":generateContent", Format: "gemini"}
	}
	if strings.HasPrefix(provider, "openai-compatible-") {
		base := strings.TrimRight(ttsString(metadata["baseUrl"]), "/")
		if base != "" {
			config = &providers.Transport{BaseURL: base + "/chat/completions", Format: "openai"}
		}
	}
	if config == nil {
		fail(501, "Translator executor not ported for provider: "+provider)
		return
	}
	body, ok := input["body"].(map[string]any)
	if !ok {
		fail(500, "Invalid request body")
		return
	}
	outbound := map[string]any{}
	for key, value := range body {
		outbound[key] = value
	}
	outbound["model"] = model
	// This endpoint receives the target-format body produced by translator step
	// three. Preserve all turns, tools, system instructions and generation options.
	if config.Format == "gemini" {
		delete(outbound, "model") // Gemini selects the model in the URL.
		if body["stream"] != false {
			config.BaseURL = strings.TrimSuffix(config.BaseURL, ":generateContent") + ":streamGenerateContent?alt=sse"
		}
		delete(outbound, "stream")
	} else if config.Format == "claude" {
		requestHeaders := make(map[string]string, len(config.Headers)+1)
		for key, value := range config.Headers {
			requestHeaders[key] = value
		}
		requestHeaders["anthropic-version"] = "2023-06-01"
		config.Headers = requestHeaders
	}
	encoded, err := json.Marshal(outbound)
	if err != nil {
		fail(500, err.Error())
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), "POST", config.BaseURL, bytes.NewReader(encoded))
	if err != nil {
		fail(500, err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	for key, value := range config.Headers {
		request.Header.Set(key, value)
	}
	token := ttsString(firstNonEmpty(connection["apiKey"], connection["accessToken"]))
	if config.Format == "gemini" {
		query := request.URL.Query()
		query.Set("key", token)
		request.URL.RawQuery = query.Encode()
	} else if !config.NoAuth {
		header, prefix := "Authorization", "Bearer "
		if config.Auth != nil {
			name, scheme := config.Auth.Header, config.Auth.Scheme
			if ttsString(connection["apiKey"]) != "" && config.Auth.APIKey != nil {
				name, scheme = config.Auth.APIKey.Header, config.Auth.APIKey.Scheme
			} else if ttsString(connection["accessToken"]) != "" && config.Auth.OAuth != nil {
				name, scheme = config.Auth.OAuth.Header, config.Auth.OAuth.Scheme
			}
			if name != "" {
				header = name
				if scheme != "bearer" {
					prefix = ""
				}
			}
		}
		request.Header.Set(header, prefix+token)
	}
	client, relay, cleanup, err := connectionHTTP(r.Context(), metadata, t.GetPool, t.Client)
	if err != nil {
		fail(500, err.Error())
		return
	}
	defer cleanup()
	if err := rewriteConnectionRelay(request, relay); err != nil {
		fail(500, err.Error())
		return
	}
	response, err := client.Do(request)
	if err != nil {
		fail(500, err.Error())
		return
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		patch, refreshErr := t.refreshConnection(r.Context(), provider, connection)
		if refreshErr != nil {
			response.Body.Close()
			fail(500, refreshErr.Error())
			return
		}
		if patch != nil {
			response.Body.Close()
			retry := request.Clone(r.Context())
			retry.Body, err = request.GetBody()
			if err != nil {
				fail(500, err.Error())
				return
			}
			// OAuth refresh must update the bearer token, never overwrite a
			// separately configured API key selected by the auth descriptor.
			if ttsString(connection["apiKey"]) == "" {
				retry.Header.Set("Authorization", "Bearer "+ttsString(patch["accessToken"]))
			}
			response, err = client.Do(retry)
			if err != nil {
				fail(500, err.Error())
				return
			}
		}
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		details, _ := io.ReadAll(response.Body)
		nodeReply(w, response.StatusCode, map[string]any{"success": false, "error": fmt.Sprintf("Provider error: %d", response.StatusCode), "details": string(details)})
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	buffer := make([]byte, 32*1024)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		if readErr != nil {
			return
		}
		select {
		case <-r.Context().Done():
			return
		default:
		}
	}
}

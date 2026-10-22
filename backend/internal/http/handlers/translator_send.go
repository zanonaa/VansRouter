package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	if !config.NoAuth {
		header, prefix := "Authorization", "Bearer "
		if config.Auth != nil && config.Auth.Header != "" {
			header = config.Auth.Header
			if config.Auth.Scheme != "bearer" {
				prefix = ""
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

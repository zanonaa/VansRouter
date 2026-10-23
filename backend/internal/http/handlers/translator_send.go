package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// TranslatorSend executes OpenAI-format transports with actual streaming. Other
// formats remain explicit pending contracts rather than silently mis-translated.
type TranslatorSend struct {
	Connections  NodeConnections
	Client       *http.Client
	GetPool      connectionPoolLookup
	refreshMu    sync.Mutex
	refreshCalls map[[32]byte]*refreshCall
}

func isHTMLContentType(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "text/html")
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
	format := providers.TargetFormat(provider)
	if format != "openai" && format != "claude" && format != "gemini" {
		fail(501, "Translator executor not ported for provider: "+provider)
		return
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
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
	if format == "gemini" {
		delete(outbound, "model")
		delete(outbound, "stream")
	}
	target, headers, err := translatorPlan(provider, model, body["stream"] != false, connection)
	if err != nil {
		fail(501, err.Error())
		return
	}
	encoded, err := json.Marshal(outbound)
	if err != nil {
		fail(500, err.Error())
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), "POST", target, bytes.NewReader(encoded))
	if err != nil {
		fail(500, err.Error())
		return
	}
	for key, value := range headers {
		request.Header.Set(key, value)
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
	// Upstream 2xx com HTML é landing/erro de rota, nunca uma resposta de
	// chat válida — sem esta barreira o HTML era repassado como "sucesso"
	// e o combo não fazia fallback (achado no teste real dos combos).
	if contentType := response.Header.Get("Content-Type"); response.StatusCode >= 200 && response.StatusCode < 300 && isHTMLContentType(contentType) {
		response.Body.Close()
		nodeReply(w, 502, map[string]any{"success": false, "error": fmt.Sprintf("Provider %s returned HTML instead of a chat response (content-type: %s)", provider, contentType)})
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		details, _ := io.ReadAll(response.Body)
		nodeReply(w, response.StatusCode, map[string]any{"success": false, "error": fmt.Sprintf("Provider error: %d", response.StatusCode), "details": string(details)})
		return
	}
	// CT do upstream é preservado: com stream:false a resposta é JSON e o
	// content-type fixo em SSE quebrava clientes JSON (achado no teste real
	// dos combos — zn-mimo via tokenharbor). Sem CT do upstream, mantém SSE.
	if upstreamType := response.Header.Get("Content-Type"); upstreamType != "" {
		w.Header().Set("Content-Type", upstreamType)
	} else {
		w.Header().Set("Content-Type", "text/event-stream")
	}
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

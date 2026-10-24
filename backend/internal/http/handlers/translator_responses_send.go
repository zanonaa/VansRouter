package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// SendResponses atende qualquer provider cujo TargetFormat seja
// "openai-responses" sem executor dedicado (ex.: perplexity-agent). Codex e
// Grok CLI continuam nos executores próprios por causa de fingerprint,
// headers e sessão. O corpo é montado a partir do intermediário OpenAI e a
// resposta é convertida para o formato do cliente por writeCodexResponse.
func (t *TranslatorSend) SendResponses(w http.ResponseWriter, r *http.Request, provider, model string, body map[string]any, source string, stream bool) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	// Codex/Grok já têm executores dedicados; aqui só os formatos que o
	// writeCodexResponse sabe converter de volta.
	if source != "openai" && source != "openai-responses" {
		fail(400, "Responses executor does not support source format: "+source)
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
	usageSelected(r.Context(), connection)

	intermediate := body
	if source == "openai" {
		intermediate, err = translateSource(body, source)
		if err != nil {
			fail(400, err.Error())
			return
		}
	}
	targetBody, err := openAIResponsesTarget(intermediate, model)
	if err != nil {
		fail(400, err.Error())
		return
	}
	t.TokenSaver.Apply(r.Context(), r, targetBody, "openai-responses", model, provider)
	encoded, err := json.Marshal(targetBody)
	if err != nil {
		fail(500, err.Error())
		return
	}
	target, headers, err := translatorPlan(provider, model, true, connection)
	if err != nil {
		fail(501, err.Error())
		return
	}
	headers["Accept"] = "text/event-stream"

	metadata, _ := connection["providerSpecificData"].(map[string]any)
	newRequest := func() (*http.Request, error) {
		request, err := http.NewRequestWithContext(r.Context(), "POST", target, bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		return request, nil
	}
	client, relay, cleanup, err := connectionHTTP(r.Context(), metadata, t.GetPool, t.Client)
	if err != nil {
		fail(500, err.Error())
		return
	}
	defer cleanup()

	refreshTried := false
	for {
		request, err := newRequest()
		if err != nil {
			fail(500, err.Error())
			return
		}
		if err := rewriteConnectionRelay(request, relay); err != nil {
			fail(500, err.Error())
			return
		}
		response, err := client.Do(request)
		if err != nil {
			fail(500, err.Error())
			return
		}
		if (response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden) && !refreshTried {
			refreshTried = true
			patch, refreshErr := t.refreshConnection(r.Context(), provider, connection)
			if refreshErr != nil {
				response.Body.Close()
				fail(500, refreshErr.Error())
				return
			}
			if patch != nil {
				response.Body.Close()
				if ttsString(connection["apiKey"]) == "" {
					headers["Authorization"] = "Bearer " + ttsString(patch["accessToken"])
				}
				continue
			}
		}
		if contentType := response.Header.Get("Content-Type"); response.StatusCode >= 200 && response.StatusCode < 300 && isHTMLContentType(contentType) {
			response.Body.Close()
			nodeReply(w, 502, map[string]any{"success": false, "error": fmt.Sprintf("Provider %s returned HTML instead of a chat response (content-type: %s)", provider, contentType)})
			return
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			details, _ := io.ReadAll(response.Body)
			response.Body.Close()
			nodeReply(w, response.StatusCode, map[string]any{"success": false, "error": fmt.Sprintf("Provider error: %d", response.StatusCode), "details": string(details)})
			return
		}
		t.writeCodexResponse(w, source, stream, model, response.Body)
		response.Body.Close()
		return
	}
}

// ResponsesSupported informa se o provider usa a Responses API e pode ser
// atendido pelo executor genérico (os dedicados são roteados antes).
func ResponsesSupported(provider string) bool {
	return providers.TargetFormat(provider) == "openai-responses"
}

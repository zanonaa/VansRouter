package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
)

func geminiCLIArch() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x64"
	case "386":
		return "x86"
	default:
		return runtime.GOARCH
	}
}

// SendGeminiCLI porta o executor gemini-cli.js: o Cloud Code Assist recebe o
// corpo Gemini embrulhado em {project, model, request} e responde em SSE
// Gemini. O cliente pode ser OpenAI, Claude ou Responses; as conversões de
// resposta passam pelo mesmo streamConversion.
func (t *TranslatorSend) SendGeminiCLI(w http.ResponseWriter, r *http.Request, provider, model string, body map[string]any, source string, stream bool) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
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
	if source != "openai" {
		intermediate, err = translateSource(body, source)
		if err != nil {
			fail(400, err.Error())
			return
		}
	}
	geminiBody, err := translateTarget(intermediate, "gemini", model)
	if err != nil {
		fail(400, err.Error())
		return
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	project := firstNonEmptyString(metadata["projectId"], metadata["quota_project_id"], connection["projectId"])
	wrapper := map[string]any{"model": model, "request": geminiBody}
	if project != "" {
		wrapper["project"] = project
	}
	t.TokenSaver.Apply(r.Context(), r, wrapper, "gemini-cli", model, provider)
	encoded, err := json.Marshal(wrapper)
	if err != nil {
		fail(500, err.Error())
		return
	}
	target, headers, err := translatorPlan(provider, model, stream, connection)
	if err != nil {
		fail(501, err.Error())
		return
	}
	if stream {
		target = strings.TrimRight(target, "/") + ":streamGenerateContent?alt=sse"
	} else {
		target = strings.TrimRight(target, "/") + ":generateContent"
	}
	headers["User-Agent"] = fmt.Sprintf("GeminiCLI/0.34.0/%s (%s; %s; terminal)", model, runtime.GOOS, geminiCLIArch())
	headers["X-Goog-Api-Client"] = "google-genai-sdk/1.41.0 gl-node/v22.19.0"
	if stream {
		headers["Accept"] = "text/event-stream"
	} else {
		headers["Accept"] = "application/json"
	}
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
			if refreshErr == nil && patch != nil {
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
		if stream {
			conversion := &streamConversion{destination: w, source: source, target: "gemini", model: model, textBlock: -1, thinkingBlock: -1, serverTool: -1}
			_, _ = io.Copy(conversion, response.Body)
			conversion.finish()
		} else {
			conversion := &convertedResponse{destination: w, source: source, target: "gemini", model: model}
			_, _ = io.Copy(conversion, response.Body)
			conversion.finish()
		}
		response.Body.Close()
		return
	}
}

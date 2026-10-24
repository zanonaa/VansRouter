package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSendGeminiCLIWrapsRequestAndConvertsSSE(t *testing.T) {
	var received map[string]any
	sender := &TranslatorSend{
		Connections: sendConnections{items: []map[string]any{{
			"isActive":             true,
			"accessToken":          "oauth-token",
			"providerSpecificData": map[string]any{"projectId": "proj-1"},
		}}},
		Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://cloudcode-pa.googleapis.com/v1internal:streamGenerateContent?alt=sse" {
				t.Fatalf("url: %s", r.URL)
			}
			if r.Header.Get("Authorization") != "Bearer oauth-token" {
				t.Fatalf("auth: %s", r.Header.Get("Authorization"))
			}
			if !strings.HasPrefix(r.Header.Get("User-Agent"), "GeminiCLI/0.34.0/gemini-3") {
				t.Fatalf("ua: %s", r.Header.Get("User-Agent"))
			}
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatal(err)
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"ok\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":1}}\n\n")),
			}, nil
		})},
	}
	w := httptest.NewRecorder()
	sender.SendGeminiCLI(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), "gemini-cli", "gemini-3", map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		"stream":   true,
	}, "openai", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"content":"ok"`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if received["project"] != "proj-1" || received["model"] != "gemini-3" {
		t.Fatalf("wrapper: %v", received)
	}
	request, _ := received["request"].(map[string]any)
	if request["contents"] == nil {
		t.Fatalf("request não convertido: %v", received)
	}
}

func TestSendCommandCodeConvertsNDJSON(t *testing.T) {
	var received map[string]any
	sender := &TranslatorSend{
		Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "user_test"}}},
		Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://api.commandcode.ai/alpha/generate" {
				t.Fatalf("url: %s", r.URL)
			}
			if r.Header.Get("x-session-id") == "" || r.Header.Get("x-command-code-version") != "0.25.7" {
				t.Fatalf("headers: %v", r.Header)
			}
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatal(err)
			}
			return &http.Response{
				StatusCode: 200,
				Body: io.NopCloser(strings.NewReader(
					"{\"type\":\"text-delta\",\"text\":\"ok\"}\n" +
						"{\"type\":\"finish-step\",\"finishReason\":\"stop\",\"usage\":{\"promptTokens\":2,\"completionTokens\":1}}\n" +
						"{\"type\":\"finish\"}\n")),
			}, nil
		})},
	}
	w := httptest.NewRecorder()
	sender.SendCommandCode(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), "commandcode", "deepseek-v4", map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		"stream":   true,
	}, "openai", true)
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, `"content":"ok"`) || !strings.Contains(body, `"finish_reason":"stop"`) || !strings.Contains(body, "[DONE]") {
		t.Fatalf("%d %s", w.Code, body)
	}
	params, _ := received["params"].(map[string]any)
	if params["stream"] != true || params["model"] != "deepseek-v4" {
		t.Fatalf("params: %v", received)
	}
	messages, _ := params["messages"].([]any)
	first, _ := messages[0].(map[string]any)
	if first["role"] != "user" {
		t.Fatalf("messages: %v", messages)
	}
}

func TestSendVertexRawKeyUsesQueryAndConvertsResponse(t *testing.T) {
	var rawURL string
	sender := &TranslatorSend{
		Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "vertex-key"}}},
		Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			rawURL = r.URL.String()
			if r.Header.Get("Authorization") != "" {
				t.Fatalf("API key não deve usar Bearer: %s", r.Header.Get("Authorization"))
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body: io.NopCloser(strings.NewReader(
					`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2}}`)),
			}, nil
		})},
	}
	w := httptest.NewRecorder()
	sender.SendVertex(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), "vertex", "gemini-3-pro", map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		"stream":   false,
	}, "openai", false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"content":"ok"`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Host != "aiplatform.googleapis.com" || !strings.HasSuffix(parsed.Path, "/publishers/google/models/gemini-3-pro:generateContent") || parsed.Query().Get("key") != "vertex-key" {
		t.Fatalf("url: %s", rawURL)
	}
}

package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestTranslatorSendPreservesNativePayload(t *testing.T) {
	for _, provider := range []string{"claude", "gemini"} {
		t.Run(provider, func(t *testing.T) {
			body := map[string]any{"stream": false, "temperature": 0.3, "max_tokens": 2048, "system": "Keep instructions", "messages": []any{map[string]any{"role": "user", "content": "first"}, map[string]any{"role": "assistant", "content": "second"}}, "tools": []any{map[string]any{"name": "lookup", "input_schema": map[string]any{"type": "object"}}}}
			if provider == "gemini" {
				body = map[string]any{"stream": true, "systemInstruction": map[string]any{"parts": []any{map[string]any{"text": "Keep instructions"}}}, "contents": []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "first"}}}, map[string]any{"role": "model", "parts": []any{map[string]any{"text": "second"}}}}, "generationConfig": map[string]any{"temperature": 0.3}, "tools": []any{map[string]any{"functionDeclarations": []any{map[string]any{"name": "lookup"}}}}}
			}
			expected := map[string]any{}
			for k, v := range body {
				expected[k] = v
			}
			if provider == "gemini" {
				delete(expected, "stream")
			} else {
				expected["model"] = "native-model"
			}
			// Normalize JSON numbers in both maps.
			encoded, _ := json.Marshal(expected)
			json.Unmarshal(encoded, &expected)
			calls := 0
			h := &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, expected) {
					t.Fatalf("payload changed: got %#v want %#v", got, expected)
				}
				if provider == "gemini" && (r.URL.Query().Get("key") != "upstream" || r.URL.Query().Get("alt") != "sse" || !strings.HasSuffix(r.URL.Path, ":streamGenerateContent")) {
					t.Fatalf("wrong Gemini URL: %s", r.URL)
				}
				if provider == "claude" && (r.Header.Get("anthropic-version") != "2023-06-01" || r.Header.Get("x-api-key") != "upstream") {
					t.Fatal("missing Claude headers")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: native\n\n"))}, nil
			})}}
			input, _ := json.Marshal(map[string]any{"provider": provider, "model": "native-model", "body": body})
			w := httptest.NewRecorder()
			h.Send(w, httptest.NewRequest("POST", "/", strings.NewReader(string(input))))
			if w.Code != 200 || calls != 1 {
				t.Fatalf("status %d calls %d: %s", w.Code, calls, w.Body.String())
			}
		})
	}
}

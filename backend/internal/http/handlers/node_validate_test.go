package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNodeValidationLocalUpstream(t *testing.T) {
	for _, tt := range []struct {
		kind, model  string
		models, chat int
		want         string
	}{
		{"", "", 200, 0, `"valid":true`}, {"", "", 401, 0, "API key unauthorized"}, {"", "", 404, 0, "Provider has no /models endpoint"},
		{"", "m", 404, 200, `"method":"chat"`}, {"anthropic-compatible", "m", 404, 400, "Invalid model or bad request"},
		{"custom-embedding", "m", 0, 200, `"dimensions":3`}, {"custom-embedding", "", 0, 0, "Model ID required"},
	} {
		t.Run(tt.kind+tt.model+tt.want, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer dummy" {
					t.Error("missing bearer")
				}
				if tt.kind == "anthropic-compatible" && r.Header.Get("anthropic-version") != "2023-06-01" {
					t.Error("version")
				}
				if strings.HasSuffix(r.URL.Path, "/models") {
					w.WriteHeader(tt.models)
					return
				}
				if strings.HasSuffix(r.URL.Path, "/embeddings") {
					w.WriteHeader(tt.chat)
					_, _ = w.Write([]byte(`{"data":[{"embedding":[1,2,3]}]}`))
					return
				}
				w.WriteHeader(tt.chat)
			}))
			defer upstream.Close()
			payload, _ := json.Marshal(map[string]any{"baseUrl": upstream.URL, "apiKey": "dummy", "type": tt.kind, "modelId": tt.model})
			w := httptest.NewRecorder()
			(NodeValidation{Local: func(*http.Request) bool { return true }}).Validate(w, httptest.NewRequest("POST", "/", strings.NewReader(string(payload))))
			if w.Code != 200 || !strings.Contains(w.Body.String(), tt.want) {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}
func TestNodeValidationRemotePrivateBlocked(t *testing.T) {
	for _, target := range []string{"http://127.0.0.1:1234", "http://169.254.169.254", "http://[::1]", "http://10.0.0.1", "http://localhost", "file:///etc/passwd"} {
		payload, _ := json.Marshal(map[string]string{"baseUrl": target, "apiKey": "dummy"})
		w := httptest.NewRecorder()
		(NodeValidation{}).Validate(w, httptest.NewRequest("POST", "/", strings.NewReader(string(payload))))
		if w.Code != 400 || !strings.Contains(w.Body.String(), "URL not allowed") {
			t.Fatalf("%s: %d %s", target, w.Code, w.Body.String())
		}
	}
}

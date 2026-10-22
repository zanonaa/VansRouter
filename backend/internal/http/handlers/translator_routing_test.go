package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranslatorUsesProviderEndpointAndAuthMode(t *testing.T) {
	for _, test := range []struct{ provider, host, apiKey, accessToken, header, value string }{
		{"glm", "api.z.ai", "plan-key", "", "x-api-key", "plan-key"},
		{"claude", "api.anthropic.com", "", "oauth-token", "Authorization", "Bearer oauth-token"},
	} {
		t.Run(test.provider+test.header, func(t *testing.T) {
			calls := 0
			handler := &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": test.apiKey, "accessToken": test.accessToken}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Host != test.host || r.Header.Get(test.header) != test.value {
					t.Fatalf("wrong destination/auth: %s %s", r.URL, r.Header.Get(test.header))
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
			})}}
			request := httptest.NewRequest("POST", "/", strings.NewReader(`{"provider":"`+test.provider+`","model":"model","body":{"messages":[{"role":"user","content":"hello"}],"max_tokens":2048}}`))
			w := httptest.NewRecorder()
			handler.Send(w, request)
			if w.Code != 200 || calls != 1 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
		})
	}
}

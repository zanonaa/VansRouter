package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestClineInlineAndFallback(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("token protocol")
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["client_type"] != "extension" || body["redirect_uri"] != "http://localhost/cb" {
			t.Error(body)
		}
		_, _ = w.Write([]byte(`{"data":{"accessToken":"fallback-access","refreshToken":"fallback-refresh","userInfo":{"email":"op@example.test"}}}`))
	}))
	defer server.Close()
	for _, provider := range []string{"cline", "clinepass"} {
		t.Run(provider, func(t *testing.T) {
			var saved map[string]any
			h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
				return &providers.OAuthConfig{AuthorizeURL: server.URL + "/auth", TokenExchangeURL: server.URL + "/token", TokenURL: server.URL + "/token"}
			}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
				saved = data
				data["id"] = "saved"
				return data, nil
			}}
			auth := deviceRoute(&h, "GET", provider, "authorize", "")
			var envelope map[string]any
			_ = json.Unmarshal(auth.Body.Bytes(), &envelope)
			target, _ := url.Parse(envelope["authUrl"].(string))
			if target.Query().Get("client_type") != "extension" || envelope["flowType"] != "authorization_code" || envelope["codeChallenge"] == nil {
				t.Fatal(envelope)
			}
			inline := base64.StdEncoding.EncodeToString([]byte(`{"accessToken":"inline-access","refreshToken":"inline-refresh","email":"inline@example.test","firstName":"Op"} trailing`))
			payload, _ := json.Marshal(map[string]string{"code": inline, "redirectUri": "http://localhost/cb", "codeVerifier": "required-for-clinepass"})
			before := calls
			w := deviceRoute(&h, "POST", provider, "exchange", string(payload))
			if w.Code != 200 || calls != before || saved["accessToken"] != "inline-access" || saved["providerSpecificData"].(map[string]any)["firstName"] != "Op" {
				t.Fatalf("inline: %d %s %v", w.Code, w.Body.String(), saved)
			}
			w = deviceRoute(&h, "POST", provider, "exchange", `{"code":"non-base64-code","redirectUri":"http://localhost/cb","codeVerifier":"verifier"}`)
			if w.Code != 200 || calls != before+1 || saved["accessToken"] != "fallback-access" || saved["email"] != "op@example.test" || strings.Contains(w.Body.String(), "fallback-access") {
				t.Fatalf("fallback: %d %s %v", w.Code, w.Body.String(), saved)
			}
		})
	}
}

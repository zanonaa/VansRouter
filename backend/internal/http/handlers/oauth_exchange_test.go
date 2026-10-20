package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestOAuthExchangeRealTransport(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			called := false
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if provider == "claude" {
					var body map[string]string
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if body["code"] != "auth-code" || body["state"] != "embedded" || r.Header.Get("Content-Type") != "application/json" {
						t.Errorf("claude body: %v", body)
					}
				} else {
					_ = r.ParseForm()
					if r.Form.Get("code") != "auth-code" || r.Form.Get("code_verifier") != "verifier" {
						t.Error(r.Form)
					}
				}
				_, _ = w.Write([]byte(`{"access_token":"dummy-access","refresh_token":"dummy-refresh","expires_in":3600}`))
			}))
			defer upstream.Close()
			var saved map[string]any
			h := OAuthEngine{Client: upstream.Client(), Config: func(string) *providers.OAuthConfig {
				return &providers.OAuthConfig{ClientID: "dummy", TokenURL: upstream.URL, AuthorizeURL: upstream.URL + "/authorize", CodeChallengeMethod: "S256", Scope: "openid email"}
			}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
				saved = data
				data["id"] = "test-id"
				return data, nil
			}}
			code := "auth-code"
			if provider == "claude" {
				code += "#embedded"
			}
			payload, _ := json.Marshal(map[string]string{"code": code, "redirectUri": "http://localhost/cb", "codeVerifier": "verifier"})
			r := httptest.NewRequest("POST", "/", strings.NewReader(string(payload)))
			route := chi.NewRouteContext()
			route.URLParams.Add("provider", provider)
			r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
			w := httptest.NewRecorder()
			h.Exchange(w, r)
			if w.Code != 200 || !called || saved["accessToken"] != "dummy-access" || saved["refreshToken"] != "dummy-refresh" || saved["expiresAt"] == nil {
				t.Fatalf("exchange %d %s saved=%v", w.Code, w.Body.String(), saved)
			}
			if strings.Contains(w.Body.String(), "dummy-access") {
				t.Fatal("token leaked")
			}
			authURL, _, verifier, err := h.buildAuthURL(provider, "http://localhost/cb")
			if err != nil {
				t.Fatal(err)
			}
			parsed, _ := url.Parse(authURL)
			if parsed.Query().Get("code_challenge") == "" || verifier == "" {
				t.Fatal(authURL)
			}
		})
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestCodebuddyRegistryAndProtocol(t *testing.T) {
	for _, provider := range []string{"codebuddy-cn", "codebuddy-intl"} {
		t.Run(provider, func(t *testing.T) {
			original := providers.GetOAuthConfig(provider)
			if original == nil || original.StateURL == "" || original.Platform == "" || original.PollInterval <= 0 || original.UserAgent == "" {
				t.Fatalf("incomplete registry: %+v", original)
			}
			if provider == "codebuddy-cn" && original.Platform != "CLI" {
				t.Fatal(original.Platform)
			}
			if provider == "codebuddy-intl" && original.Platform != "ide" {
				t.Fatal(original.Platform)
			}
			mode := "pending"
			savedCount := 0
			var saved map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Header.Get("User-Agent") != original.UserAgent || r.Header.Get("X-Domain") != codebuddyDomains[provider] {
					t.Error("wrong provider headers")
				}
				if r.URL.Path == "/state" {
					if r.Method != "POST" || r.URL.Query().Get("platform") != original.Platform {
						t.Error("init method/platform", r.Method, r.URL.Query())
					}
					var payload map[string]any
					if json.NewDecoder(r.Body).Decode(&payload) != nil || len(payload) != 0 {
						t.Error("init body")
					}
					if mode == "missing-code" {
						_, _ = w.Write([]byte(`{"data":{"state":"s","authUrl":"https://example.test"}}`))
						return
					}
					_, _ = w.Write([]byte(`{"code":0,"data":{"state":"cb-state","authUrl":"https://example.test/auth"}}`))
					return
				}
				if r.Method != "GET" || r.URL.Query().Get("state") != "cb-state" || r.Header.Get("X-No-Enterprise-Id") != "true" {
					t.Error("poll protocol")
				}
				switch mode {
				case "pending":
					_, _ = w.Write([]byte(`{"code":11217}`))
				case "failure":
					w.WriteHeader(500)
				case "missing-code":
					_, _ = w.Write([]byte(`{"data":{"accessToken":"not-valid"}}`))
				case "success":
					_, _ = w.Write([]byte(`{"code":0,"data":{"accessToken":"cb-access","expiresIn":7200}}`))
				case "fallback":
					_, _ = w.Write([]byte(`{"code":0,"data":{"accessToken":"cb-access"}}`))
				default:
					_, _ = w.Write([]byte(`{"code":1}`))
				}
			}))
			defer server.Close()
			c := *original
			c.StateURL = server.URL + "/state"
			c.TokenURL = server.URL + "/token"
			h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig { return &c }, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
				savedCount++
				saved = data
				data["id"] = "saved"
				return data, nil
			}}
			w := deviceRoute(&h, "GET", provider, "device-code", "")
			var init map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &init)
			if w.Code != 200 || init["interval"] != original.PollInterval/1000 || init["_isCodeBuddy"] != true || !nodeTruthy(init["codeVerifier"]) {
				t.Fatal(w.Code, init)
			}
			for _, state := range []string{"pending", "failure", "missing-code", "unknown"} {
				mode = state
				w = deviceRoute(&h, "POST", provider, "poll", `{"deviceCode":"cb-state"}`)
				if w.Code != 200 || savedCount != 0 {
					t.Fatal(state, w.Code, savedCount)
				}
				var payload map[string]any
				_ = json.Unmarshal(w.Body.Bytes(), &payload)
				if state == "pending" && payload["pending"] != true {
					t.Fatal(payload)
				}
				if state == "missing-code" && payload["error"] != "unknown_error" {
					t.Fatal(payload)
				}
			}
			for _, state := range []string{"success", "fallback"} {
				mode = state
				w = deviceRoute(&h, "POST", provider, "poll", `{"deviceCode":"cb-state"}`)
				if w.Code != 200 || saved["accessToken"] != "cb-access" || saved["refreshToken"] != "" || strings.Contains(w.Body.String(), "cb-access") {
					t.Fatal(w.Code, saved)
				}
				want := 7200.0
				if state == "fallback" {
					want = 86400
				}
				if saved["expiresIn"] != want {
					t.Fatal(saved)
				}
				expiry, err := time.Parse(time.RFC3339Nano, saved["expiresAt"].(string))
				if err != nil || time.Until(expiry) < time.Duration(want-5)*time.Second {
					t.Fatal("expiry", saved)
				}
			}
			mode = "missing-code"
			w = deviceRoute(&h, "GET", provider, "device-code", "")
			if w.Code != 500 {
				t.Fatal("missing code accepted", w.Body.String())
			}
		})
	}
}

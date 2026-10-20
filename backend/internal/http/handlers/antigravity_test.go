package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestAntigravityFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
				t.Error("form")
			}
			_, _ = w.Write([]byte(`{"access_token":"ag-access","refresh_token":"ag-refresh","expires_in":3600,"scope":"cloud-platform"}`))
		case "/user":
			if r.Header.Get("x-request-source") != "local" {
				t.Error("source header")
			}
			_, _ = w.Write([]byte(`{"email":"ag@example.test"}`))
		case "/load":
			if r.Header.Get("x-request-source") != "local" {
				t.Error("load source")
			}
			_, _ = w.Write([]byte(`{"cloudaicompanionProject":{"id":"ag-proj"},"allowedTiers":[{"id":"other"},{"id":"tier-9","isDefault":true}]}`))
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer server.Close()
	var saved map[string]any
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{ClientID: "ag", ClientSecret: "ags", AuthorizeURL: server.URL + "/auth", TokenURL: server.URL + "/token", UserInfoURL: server.URL + "/user", ProjectURL: server.URL + "/load"}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "antigravity", "authorize", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "access_type=offline") || !strings.Contains(w.Body.String(), "prompt=consent") || !strings.Contains(w.Body.String(), `"flowType":"authorization_code"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	payload := `{"code":"c","redirectUri":"http://localhost/cb","codeVerifier":"verifier"}`
	w = deviceRoute(&h, "POST", "antigravity", "exchange", payload)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"success":true`) || strings.Contains(w.Body.String(), "ag-access") {
		t.Fatalf("exchange: %d %s", w.Code, w.Body.String())
	}
	if saved["projectId"] != "ag-proj" || saved["email"] != "ag@example.test" || saved["refreshToken"] != "ag-refresh" {
		t.Fatal(saved)
	}
}

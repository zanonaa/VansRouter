package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestKimchiTokenFlow(t *testing.T) {
	validation := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/validate":
			validation++
			if r.Header.Get("Authorization") != "Bearer browser-token" {
				t.Error("bearer")
			}
			w.WriteHeader(200)
		case "/user":
			if r.Header.Get("Authorization") != "Bearer browser-token" {
				t.Error("user bearer")
			}
			_, _ = w.Write([]byte(`{"id":42,"username":"kimchiop","name":"Kimchi Operator","email":"kimchi@example.test"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	var saved map[string]any
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{AuthorizeURL: "https://app.kimchi.dev", UserInfoURL: server.URL + "/validate", TokenExchangeURL: server.URL + "/user"}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "kimchi", "authorize", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "/cli-auth?callback=") || !strings.Contains(w.Body.String(), `"flowType":"browser_token"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = deviceRoute(&h, "POST", "kimchi", "exchange", `{}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Missing Kimchi token") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = deviceRoute(&h, "POST", "kimchi", "exchange", `{"code":"  browser-token  ","redirectUri":"http://localhost/cb"}`)
	if w.Code != 200 || validation != 1 || saved["accessToken"] != "browser-token" || saved["refreshToken"] != nil || strings.Contains(w.Body.String(), "browser-token") {
		t.Fatalf("exchange: %d %s %v", w.Code, w.Body.String(), saved)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["authMethod"] != "browser_token" || specific["userId"] != "42" || specific["username"] != "kimchiop" {
		t.Fatal(specific)
	}
	if saved["email"] != "kimchi@example.test" || saved["displayName"] != "Kimchi Operator" {
		t.Fatal(saved)
	}
	_ = json.Marshal
}

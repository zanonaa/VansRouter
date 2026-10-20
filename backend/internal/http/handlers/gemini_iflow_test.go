package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestGeminiAndIflowOAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			_ = r.ParseForm()
			if r.Header.Get("Authorization") != "" {
				_, _ = w.Write([]byte(`{"access_token":"dummy-access","refresh_token":"dummy-refresh","expires_in":3600}`))
			} else {
				_, _ = w.Write([]byte(`{"access_token":"dummy-access","refresh_token":"dummy-refresh","expires_in":3600,"scope":"cloud-platform"}`))
			}
		case "/user":
			_, _ = w.Write([]byte(`{"email":"op@example.test","name":"Operator"}`))
		case "/project":
			_, _ = w.Write([]byte(`{"cloudaicompanionProject":{"id":"proj-1"}}`))
		case "/iflow-user":
			if r.URL.Query().Get("accessToken") != "dummy-access" {
				t.Error("iflow access token missing")
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"apiKey":"if-key","email":"if@example.test","nickname":"iFlowOp"}}`))
		case "/iflow-fail":
			_, _ = w.Write([]byte(`{"success":false,"message":"bad token"}`))
		}
	}))
	defer server.Close()

	saved := map[string]map[string]any{}
	config := func(id string) *providers.OAuthConfig {
		switch id {
		case "gemini-cli":
			return &providers.OAuthConfig{ClientID: "g", ClientSecret: "gs", TokenURL: server.URL + "/token", AuthorizeURL: server.URL + "/auth", UserInfoURL: server.URL + "/user", ProjectURL: server.URL + "/project", Scopes: []string{"cloud-platform", "email"}}
		case "iflow":
			return &providers.OAuthConfig{ClientID: "i", ClientSecret: "is", TokenURL: server.URL + "/token", AuthorizeURL: server.URL + "/auth", UserInfoURL: server.URL + "/iflow-user", ExtraParams: map[string]string{"loginMethod": "phone", "type": "login"}}
		}
		return nil
	}
	h := OAuthEngine{Client: server.Client(), Config: config, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		provider, _ := data["provider"].(string)
		saved[provider] = data
		data["id"] = "saved-" + provider
		return data, nil
	}}

	r2 := httptest.NewRequest("POST", "/", strings.NewReader(`{"code":"c","redirectUri":"http://localhost/cb","codeVerifier":"required-by-route"}`))
	setChiProvider(r2, "gemini-cli")
	w := httptest.NewRecorder()
	h.Exchange(w, r2)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "success") {
		t.Fatalf("gemini exchange: %d %s", w.Code, w.Body.String())
	}
	if saved["gemini-cli"]["projectId"] != "proj-1" || saved["gemini-cli"]["email"] != "op@example.test" {
		t.Fatal(saved["gemini-cli"])
	}

	r3 := httptest.NewRequest("POST", "/", strings.NewReader(`{"code":"c","redirectUri":"http://localhost/cb","codeVerifier":"required-by-route"}`))
	setChiProvider(r3, "iflow")
	w = httptest.NewRecorder()
	h.Exchange(w, r3)
	if w.Code != 200 {
		t.Fatalf("iflow exchange: %d %s", w.Code, w.Body.String())
	}
	if saved["iflow"]["apiKey"] != "if-key" || saved["iflow"]["email"] != "if@example.test" || saved["iflow"]["displayName"] != "iFlowOp" {
		t.Fatal(saved["iflow"])
	}

	failConfig := func(id string) *providers.OAuthConfig {
		if id == "iflow" {
			return &providers.OAuthConfig{ClientID: "i", ClientSecret: "is", TokenURL: server.URL + "/token", UserInfoURL: server.URL + "/iflow-fail"}
		}
		return nil
	}
	hFail := OAuthEngine{Client: server.Client(), Config: failConfig, CreateConnection: h.CreateConnection}
	r4 := httptest.NewRequest("POST", "/", strings.NewReader(`{"code":"c","redirectUri":"http://localhost/cb","codeVerifier":"required-by-route"}`))
	setChiProvider(r4, "iflow")
	w = httptest.NewRecorder()
	hFail.Exchange(w, r4)
	if w.Code != 500 || !strings.Contains(w.Body.String(), "User info request failed") {
		t.Fatalf("iflow fail: %d %s", w.Code, w.Body.String())
	}
}

func setChiProvider(r *http.Request, provider string) {
	route := chi.NewRouteContext()
	route.URLParams.Add("provider", provider)
	route.URLParams.Add("action", "exchange")
	*r = *r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
}

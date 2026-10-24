package handlers

import (
	"context"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestGitlabSelfHostedFlow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_ = r.ParseForm()
			if r.Form.Get("client_id") != "self-hosted-id" || r.Form.Get("client_secret") != "secret" || r.Form.Get("code_verifier") == "" {
				w.WriteHeader(400)
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"gl-access","refresh_token":"gl-refresh","expires_in":7200,"scope":"api"}`))
		case "/api/v4/user":
			if r.Header.Get("Authorization") != "Bearer gl-access" {
				w.WriteHeader(401)
				return
			}
			_, _ = w.Write([]byte(`{"id":7,"username":"gloperator","name":"GitLab Operator","email":"gl@example.test","public_email":""}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	var saved map[string]any
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		original := providers.GetOAuthConfig("gitlab")
		if original == nil || original.TokenURLPath != "/oauth/token" || original.AuthorizeURLPath != "/oauth/authorize" || original.DefaultBaseURL != "https://gitlab.com" {
			t.Fatal("incomplete GitLab snapshot", original)
		}
		config := *original
		// TokenURL absoluto é proposital: deve usar TokenURLPath ao concatenar base.
		config.TokenURL = "https://gitlab.com/oauth/token"
		return &config
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}

	authorizeReq := httptest.NewRequest("GET", "/api/oauth/gitlab/authorize?baseUrl="+server.URL+"&clientId=self-hosted-id&redirect_uri=http://localhost/cb", nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("provider", "gitlab")
	route.URLParams.Add("action", "authorize")
	authorizeReq = authorizeReq.WithContext(context.WithValue(authorizeReq.Context(), chi.RouteCtxKey, route))
	w := httptest.NewRecorder()
	h.Route(w, authorizeReq)
	if w.Code != 200 || !strings.Contains(w.Body.String(), server.URL+"/oauth/authorize?") || !strings.Contains(w.Body.String(), "code_challenge") {
		t.Fatal(w.Code, w.Body.String())
	}

	w = deviceRoute(&h, "POST", "gitlab", "exchange", `{"code":"c","redirectUri":"http://localhost/cb","codeVerifier":"v","meta":{"baseUrl":"`+server.URL+`","clientId":"self-hosted-id","clientSecret":"secret"}}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"success":true`) || strings.Contains(w.Body.String(), "gl-access") {
		t.Fatalf("exchange: %d %s", w.Code, w.Body.String())
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["baseUrl"] != server.URL || specific["clientId"] != "self-hosted-id" || specific["username"] != "gloperator" || specific["authKind"] != "oauth" {
		t.Fatal(specific)
	}
	if saved["email"] != nil {
		t.Fatal("email should be in providerSpecificData not top-level for gitlab")
	}
}

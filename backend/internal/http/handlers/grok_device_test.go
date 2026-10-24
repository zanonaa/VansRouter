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

func TestGrokCLIProtocol(t *testing.T) {
	t.Setenv("GROK_CLI_VERSION", "1.4.2")
	state := "pending"
	polls := 0
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("User-Agent") != "grok-pager/1.4.2 grok-shell/1.4.2 (linux; x86_64)" {
			t.Error("user-agent")
		}
		switch r.URL.Path {
		case "/device":
			_ = r.ParseForm()
			if r.Form.Get("referrer") != "grok-build" || r.Form.Get("scope") != "openid email" || r.Form.Get("code_challenge") != "" {
				t.Error(r.Form)
			}
			_, _ = w.Write([]byte(`{"device_code":"device","interval":5}`))
		case "/token":
			polls++
			_ = r.ParseForm()
			if r.Form.Get("code_verifier") != "" || r.Form.Get("device_code") != "device" {
				t.Error(r.Form)
			}
			if state == "pending" {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			} else if state == "slow" {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"error":"slow_down"}`))
			} else {
				_, _ = w.Write([]byte(`{"access_token":"grok-access","refresh_token":"grok-refresh","expires_in":2400,"scope":"api:access"}`))
			}
		case "/user":
			if r.Header.Get("x-xai-token-auth") != "xai-grok-cli" || r.Header.Get("x-grok-client-version") != "1.4.2" || r.Header.Get("Authorization") != "Bearer grok-access" {
				t.Error("profile headers")
			}
			_, _ = w.Write([]byte(`{"email":"op@example.test","userId":"u","firstName":"Op","lastName":"Erator","hasGrokCodeAccess":false,"subscriptionTier":"pro"}`))
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{ClientID: "client", DeviceCodeURL: server.URL + "/device", TokenURL: server.URL + "/token", UserInfoURL: server.URL + "/user", Scope: "openid email", Referrer: "grok-build"}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "grok-cli", "device-code", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "codeVerifier") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = deviceRoute(&h, "POST", "grok-cli", "poll", `{"deviceCode":"device"}`)
	if w.Code != 400 || polls != 0 {
		t.Fatal("missing verifier not rejected")
	}
	for _, mode := range []string{"pending", "slow"} {
		state = mode
		w = deviceRoute(&h, "POST", "grok-cli", "poll", `{"deviceCode":"device","codeVerifier":"unused"}`)
		var result map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		if w.Code != 200 || result["pending"] != (mode == "pending") || saved != nil {
			t.Fatal(result, saved)
		}
	}
	state = "success"
	w = deviceRoute(&h, "POST", "grok-cli", "poll", `{"deviceCode":"device","codeVerifier":"unused"}`)
	if w.Code != 200 || saved["expiresAt"] == nil || saved["email"] != "op@example.test" || saved["displayName"] != "Op Erator" || strings.Contains(w.Body.String(), "grok-access") {
		t.Fatal(w.Code, saved)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["hasGrokCodeAccess"] != false || specific["subscriptionTier"] != "pro" || specific["userId"] != "u" {
		t.Fatal(specific)
	}
	t.Setenv("GROK_CLI_VERSION", "invalid")
	if grokVersion() != "1.0.44" {
		t.Fatal("version fallback")
	}
}

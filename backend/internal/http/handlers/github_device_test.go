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

func TestGithubDeviceAndPoll(t *testing.T) {
	pending := true
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device":
			_ = r.ParseForm()
			if r.Form.Get("scope") != "read:user" {
				t.Error(r.Form)
			}
			_, _ = w.Write([]byte(`{"device_code":"dummy-device","user_code":"AAAA","verification_uri":"https://example.test","interval":5}`))
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("device_code") != "dummy-device" {
				t.Error(r.Form)
			}
			if pending {
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			} else {
				_, _ = w.Write([]byte(`{"access_token":"dummy-access","refresh_token":"dummy-refresh","expires_in":3600}`))
			}
		case "/copilot":
			calls++
			if r.Header.Get("Authorization") != "Bearer dummy-access" {
				t.Error("missing bearer")
			}
			_, _ = w.Write([]byte(`{"token":"dummy-copilot","expires_at":1234}`))
		case "/user":
			calls++
			_, _ = w.Write([]byte(`{"id":42,"login":"operator","name":"Operator","email":"op@example.test"}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	var saved map[string]any
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{ClientID: "dummy-client", DeviceScope: "read:user", DeviceCodeURL: server.URL + "/device", TokenURL: server.URL + "/token", CopilotTokenURL: server.URL + "/copilot", UserInfoURL: server.URL + "/user", APIVersion: "2022-11-28", UserAgent: "test"}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := httptest.NewRecorder()
	h.githubDevice(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "codeVerifier") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.githubPoll(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"deviceCode":"dummy-device"}`)))
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	if result["pending"] != true || saved != nil || calls != 0 {
		t.Fatal(result, saved, calls)
	}
	pending = false
	w = httptest.NewRecorder()
	h.githubPoll(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"deviceCode":"dummy-device"}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"success":true`) || strings.Contains(w.Body.String(), "dummy-access") {
		t.Fatal(w.Code, w.Body.String())
	}
	if saved["accessToken"] != "dummy-access" || saved["name"] != "operator" || saved["expiresAt"] == nil || calls != 2 {
		t.Fatal(saved, calls)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["copilotToken"] != "dummy-copilot" || specific["githubUserId"] != float64(42) {
		t.Fatal(specific)
	}
}

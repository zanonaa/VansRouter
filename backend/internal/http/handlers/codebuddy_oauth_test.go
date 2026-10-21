package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestCodebuddyDevicePoll(t *testing.T) {
	mode := "pending"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/state" {
			if r.URL.Query().Get("platform") != "qingke-ide" {
				t.Error("platform missing")
			}
			if r.Header.Get("X-Domain") != "copilot.tencent.com" {
				t.Error("domain header")
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"state":"cb-state","authUrl":"https://copilot.tencent.com/auth"}}`))
			return
		}
		if r.Method != "GET" || r.URL.Query().Get("state") != "cb-state" {
			t.Error("poll must GET with state")
		}
		if r.Header.Get("X-No-Enterprise-Id") != "true" {
			t.Error("poll headers")
		}
		if mode == "pending" {
			_, _ = w.Write([]byte(`{"code":11217}`))
		} else {
			_, _ = w.Write([]byte(`{"code":0,"data":{"accessToken":"cb-access","refreshToken":"cb-refresh","expiresIn":7200}}`))
		}
	}))
	defer server.Close()
	var saved map[string]any
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{DeviceCodeURL: server.URL + "/state", TokenURL: server.URL + "/token", DeviceScope: "qingke-ide"}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "codebuddy-cn", "device-code", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "cb-state") || !strings.Contains(w.Body.String(), `"user_code":""`) {
		t.Fatal(w.Code, w.Body.String())
	}
	w = deviceRoute(&h, "POST", "codebuddy-cn", "poll", `{"deviceCode":"cb-state"}`)
	if w.Code != 200 || saved != nil || !strings.Contains(w.Body.String(), `"pending":true`) {
		t.Fatal(w.Body.String())
	}
	mode = "success"
	w = deviceRoute(&h, "POST", "codebuddy-cn", "poll", `{"deviceCode":"cb-state"}`)
	if w.Code != 200 || saved["accessToken"] != "cb-access" || saved["expiresIn"] != 7200.0 || strings.Contains(w.Body.String(), "cb-access") {
		t.Fatal(w.Code, saved)
	}
}

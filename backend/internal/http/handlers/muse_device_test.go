package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestMuseAccountTokenIsNotModelKey(t *testing.T) {
	mode := "pending"
	mints := 0
	waits := []time.Duration{}
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("x-api-version") != "1.0.0" {
			t.Error("api version")
		}
		switch r.URL.Path {
		case "/device":
			_ = r.ParseForm()
			if r.Form.Get("client_id") != "meta-client" || r.Form.Get("code_challenge") != "" {
				t.Error(r.Form)
			}
			_, _ = w.Write([]byte(`{"device_code":"device","user_code":"CODE"}`))
		case "/token":
			_ = r.ParseForm()
			if r.Form.Get("code_verifier") != "" || r.Form.Get("device_code") != "device" {
				t.Error(r.Form)
			}
			if mode == "pending" {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
			} else {
				_, _ = w.Write([]byte(`{"access_token":"meta-account-token"}`))
			}
		case "/key":
			mints++
			if r.Header.Get("Authorization") != "Bearer meta-account-token" {
				t.Error("mint bearer")
			}
			if mode == "inactive" {
				_, _ = w.Write([]byte(`{"is_subs_active":false}`))
				return
			}
			if mints < 3 {
				w.WriteHeader(429)
				_, _ = w.Write([]byte(`{"title":"rate limit"}`))
				return
			}
			_, _ = w.Write([]byte(`{"api_key":"LLM|model-key","user_email":" OP@EXAMPLE.TEST ","subs_tier_name":"pro"}`))
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{ClientID: "meta-client", DeviceCodeURL: server.URL + "/device", TokenURL: server.URL + "/token", MuseKeyURL: server.URL + "/key"}
	}, RetryWait: func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil }, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "muse", "device-code", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "codeVerifier") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = deviceRoute(&h, "POST", "muse", "poll", `{"deviceCode":"device","codeVerifier":"unused"}`)
	if w.Code != 200 || saved != nil || mints != 0 || !strings.Contains(w.Body.String(), `"pending":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	mode = "inactive"
	w = deviceRoute(&h, "POST", "muse", "poll", `{"deviceCode":"device","codeVerifier":"unused"}`)
	if w.Code != 500 || saved != nil || !strings.Contains(w.Body.String(), "subscription is inactive") {
		t.Fatal(w.Code, w.Body.String())
	}
	mode = "success"
	mints = 0
	w = deviceRoute(&h, "POST", "muse", "poll", `{"deviceCode":"device","codeVerifier":"unused"}`)
	if w.Code != 200 || mints != 3 || len(waits) != 2 || waits[0] != 5*time.Second || waits[1] != 10*time.Second {
		t.Fatal(w.Code, w.Body.String(), mints, waits)
	}
	if saved["accessToken"] != "LLM|model-key" || saved["refreshToken"] != nil || saved["expiresAt"] != nil || saved["email"] != "op@example.test" || strings.Contains(w.Body.String(), "model-key") {
		t.Fatal(saved, w.Body.String())
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["oauthAccessToken"] != "meta-account-token" || specific["subscriptionTier"] != "pro" {
		t.Fatal(specific)
	}
}

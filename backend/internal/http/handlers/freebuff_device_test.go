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

func TestFreebuffFingerprintFlow(t *testing.T) {
	authorized := false
	polls := 0
	fingerprint := ""
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("User-Agent") != "codebuff-cli/0.0.138" {
			t.Error("user agent")
		}
		switch r.URL.Path {
		case "/code":
			if r.Method != "POST" {
				t.Error("code method")
			}
			var input map[string]string
			_ = json.NewDecoder(r.Body).Decode(&input)
			fingerprint = input["fingerprintId"]
			if len(fingerprint) != 36 {
				t.Error("fingerprint missing")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"fingerprintId": fingerprint, "fingerprintHash": "hash", "expiresAt": 1900000000000, "loginUrl": "https://freebuff.com/login?auth_code=CODE&next=1"})
		case "/status":
			polls++
			q := r.URL.Query()
			if r.Method != "GET" || q.Get("fingerprintId") != fingerprint || q.Get("fingerprintHash") != "hash" || q.Get("expiresAt") != "1900000000000" {
				t.Error(r.Method, q)
			}
			if !authorized {
				w.WriteHeader(401)
				_, _ = w.Write([]byte(`{"error":"Authentication failed"}`))
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"user": map[string]any{"authToken": "dummy-auth", "id": "user", "email": "op@example.test", "name": "Operator", "fingerprintId": fingerprint}})
			}
		default:
			t.Error(r.URL.Path)
		}
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{BaseURL: server.URL, LoginCodePath: "/code", LoginStatusPath: "/status", OAuthTimeoutMS: 300000}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "freebuff", "device-code", "")
	var data map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &data)
	if w.Code != 200 || data["user_code"] != "CODE" || data["expires_in"] != float64(300) {
		t.Fatal(w.Code, data)
	}
	payload, _ := json.Marshal(map[string]any{"deviceCode": data["device_code"], "codeVerifier": data["codeVerifier"]})
	w = deviceRoute(&h, "POST", "freebuff", "poll", string(payload))
	if w.Code != 200 || saved != nil || polls != 1 || !strings.Contains(w.Body.String(), `"pending":true`) {
		t.Fatal(w.Code, w.Body.String())
	}
	authorized = true
	w = deviceRoute(&h, "POST", "freebuff", "poll", string(payload))
	if w.Code != 200 || saved["accessToken"] != "dummy-auth" || saved["email"] != "op@example.test" || saved["refreshToken"] != nil || strings.Contains(w.Body.String(), "dummy-auth") {
		t.Fatal(w.Code, w.Body.String(), saved)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["fingerprintId"] != fingerprint || specific["userId"] != "user" {
		t.Fatal(specific)
	}
	before := polls
	w = deviceRoute(&h, "POST", "freebuff", "poll", `{"deviceCode":"bad-json","codeVerifier":"v"}`)
	if w.Code != 200 || polls != before || !strings.Contains(w.Body.String(), `"pending":true`) {
		t.Fatal("malformed identity should remain pending")
	}
	w = deviceRoute(&h, "POST", "freebuff", "poll", `{"deviceCode":"identity"}`)
	if w.Code != 400 {
		t.Fatal("missing verifier")
	}
}

package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestQoderNonceMachineAndPolling(t *testing.T) {
	status := 202
	profileOK := true
	nonce, verifier := "", ""
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Header.Get("User-Agent") != "Go-http-client/2.0" {
			t.Error("UA")
		}
		if r.URL.Path == "/poll" {
			if r.Method != "GET" || r.URL.Query().Get("nonce") != nonce || r.URL.Query().Get("verifier") != verifier || r.URL.Query().Get("challenge_method") != "S256" {
				t.Error(r.Method, r.URL.Query())
			}
			w.WriteHeader(status)
			if status == 200 {
				_, _ = w.Write([]byte(`{"token":"dt-access","refresh_token":"refresh","user_id":"user","expires_in":0}`))
			}
			return
		}
		if r.URL.Path == "/user" {
			if !profileOK {
				w.WriteHeader(500)
				return
			}
			if r.Header.Get("Authorization") != "Bearer dt-access" {
				t.Error("profile bearer")
			}
			_, _ = w.Write([]byte(`{"name":" Operator ","email":"op@example.test","organization_id":"org"}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{LoginURL: "https://qoder.com/device/selectAccounts", DeviceTokenURL: server.URL + "/poll", UserInfoURL: server.URL + "/user"}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "qoder", "device-code", "")
	var flow map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &flow)
	nonce, _ = flow["device_code"].(string)
	verifier, _ = flow["codeVerifier"].(string)
	u, _ := url.Parse(flow["verification_uri_complete"].(string))
	sum := sha256.Sum256([]byte(verifier))
	if w.Code != 200 || u.Query().Get("challenge") != base64.RawURLEncoding.EncodeToString(sum[:]) || u.Query().Get("machine_id") != flow["_qoderMachineId"] || len(nonce) != 36 {
		t.Fatal(w.Code, flow)
	}
	payload, _ := json.Marshal(map[string]any{"deviceCode": nonce, "codeVerifier": verifier, "extraData": map[string]any{"_qoderMachineId": flow["_qoderMachineId"]}})
	for _, code := range []int{202, 404} {
		status = code
		w = deviceRoute(&h, "POST", "qoder", "poll", string(payload))
		if w.Code != 200 || saved != nil || !strings.Contains(w.Body.String(), "authorization_pending") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	status = 200
	w = deviceRoute(&h, "POST", "qoder", "poll", string(payload))
	if w.Code != 200 || saved["accessToken"] != "dt-access" || saved["expiresIn"] != 86400.0 || saved["email"] != "op@example.test" || strings.Contains(w.Body.String(), "dt-access") {
		t.Fatal(w.Code, saved)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["machineId"] != flow["_qoderMachineId"] || specific["organizationId"] != "org" {
		t.Fatal(specific)
	}
	profileOK = false
	w = deviceRoute(&h, "POST", "qoder", "poll", string(payload))
	if saved["email"] != "qoder-user-user" {
		t.Fatal("synthetic identity missing", saved)
	}
}
func TestQoderExpiryHints(t *testing.T) {
	now := time.UnixMilli(1700000000000)
	for _, tt := range []struct {
		at, seconds any
		want        int64
	}{{float64(1800000000000), nil, 1800000000000}, {"1800000000000", nil, 1800000000000}, {nil, float64(0), now.UnixMilli()}, {nil, nil, now.Add(30 * 24 * time.Hour).UnixMilli()}} {
		if got := qoderExpiry(tt.at, tt.seconds, now).UnixMilli(); got != tt.want {
			t.Fatalf("expiry %v: %d want %d", tt.at, got, tt.want)
		}
	}
}

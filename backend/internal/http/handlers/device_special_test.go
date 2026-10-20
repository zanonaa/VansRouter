package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func deviceRoute(h *OAuthEngine, method, provider, action, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/oauth/"+provider+"/"+action, strings.NewReader(body))
	route := chi.NewRouteContext()
	route.URLParams.Add("provider", provider)
	route.URLParams.Add("action", action)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
	w := httptest.NewRecorder()
	h.Route(w, r)
	return w
}
func TestQwenPKCEAndPoll(t *testing.T) {
	challenge := ""
	polls := 0
	mode := "pending"
	saves := 0
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("client_id") != "qwen-client" {
			t.Error(r.Form)
		}
		if r.URL.Path == "/device" {
			challenge = r.Form.Get("code_challenge")
			if r.Form.Get("scope") != "openid profile email model.completion" || r.Form.Get("code_challenge_method") != "S256" {
				t.Error(r.Form)
			}
			_, _ = w.Write([]byte(`{"device_code":"qwen-device","interval":5}`))
			return
		}
		polls++
		if r.Form.Get("code_verifier") == "" || r.Form.Get("device_code") != "qwen-device" {
			t.Error(r.Form)
		}
		switch mode {
		case "pending":
			_, _ = w.Write([]byte(`{"error":"authorization_pending","message":"wait"}`))
		case "slow":
			_, _ = w.Write([]byte(`{"error":"slow_down"}`))
		case "empty":
			_, _ = w.Write([]byte(`{}`))
		case "denied":
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"access_denied","error_description":"denied"}`))
		default:
			_, _ = w.Write([]byte(`{"access_token":"qwen-access","refresh_token":"qwen-refresh","expires_in":0,"resource_url":"https://resource.test"}`))
		}
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{ClientID: "qwen-client", DeviceCodeURL: server.URL + "/device", TokenURL: server.URL + "/token", Scope: "openid profile email model.completion", CodeChallengeMethod: "S256"}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saves++
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "qwen", "device-code", "")
	var data map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	verifier, _ := data["codeVerifier"].(string)
	sum := sha256.Sum256([]byte(verifier))
	if w.Code != 200 || challenge != base64.RawURLEncoding.EncodeToString(sum[:]) {
		t.Fatalf("PKCE: %d %s challenge=%s", w.Code, w.Body.String(), challenge)
	}
	if w = deviceRoute(&h, "POST", "qwen", "poll", `{"deviceCode":"qwen-device"}`); w.Code != 400 || polls != 0 {
		t.Fatal("missing verifier reached upstream")
	}
	payload, _ := json.Marshal(map[string]string{"deviceCode": "qwen-device", "codeVerifier": verifier})
	for _, state := range []string{"pending", "slow", "empty", "denied"} {
		mode = state
		w = deviceRoute(&h, "POST", "qwen", "poll", string(payload))
		if w.Code != 200 || saves != 0 {
			t.Fatalf("failure saved: %s %d", state, w.Code)
		}
		var result map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &result)
		if state == "pending" && (result["pending"] != true || result["errorDescription"] != "wait") {
			t.Fatal(result)
		}
		if state == "slow" && result["pending"] != false {
			t.Fatal(result)
		}
		if state == "empty" && result["errorDescription"] != "No access token received" {
			t.Fatal(result)
		}
	}
	mode = "success"
	w = deviceRoute(&h, "POST", "qwen", "poll", string(payload))
	if w.Code != 200 || saves != 1 || saved["accessToken"] != "qwen-access" || saved["expiresAt"] != nil {
		t.Fatalf("saved: %d %s %v", w.Code, w.Body.String(), saved)
	}
}
func TestKimiDeviceIdentityAndAlias(t *testing.T) {
	deviceID := ""
	mode := "pending"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Header.Get("X-Msh-Platform") != "9router" || r.Header.Get("X-Msh-Version") == "" || r.Header.Get("X-Msh-Device-Model") == "" {
			t.Error("missing Kimi headers")
		}
		if r.Form.Get("code_verifier") != "" || r.Form.Get("scope") != "" {
			t.Error("generic form fields leaked")
		}
		if r.URL.Path == "/device" {
			deviceID = r.Header.Get("X-Msh-Device-Id")
			_, _ = w.Write([]byte(`{"device_code":"kimi-device","user_code":"CODE","expires_in":300}`))
			return
		}
		if r.Header.Get("X-Msh-Device-Id") != deviceID {
			t.Error("device ID changed during polling")
		}
		if mode == "pending" {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
		} else {
			_, _ = w.Write([]byte(`{"access_token":"kimi-access","refresh_token":"kimi-refresh","expires_in":3600}`))
		}
	}))
	defer server.Close()
	var saved map[string]any
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{ClientID: "kimi-client", DeviceCodeURL: server.URL + "/device", TokenURL: server.URL + "/token"}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "kimi-coding", "device-code", "")
	var data map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &data)
	if w.Code != 200 || data["_kimiDeviceId"] != deviceID || deviceID == "" || data["interval"] != float64(5) || data["verification_uri_complete"] != "https://www.kimi.com/code/authorize_device?user_code=CODE" {
		t.Fatal(w.Code, data)
	}
	payload, _ := json.Marshal(map[string]any{"deviceCode": "kimi-device", "codeVerifier": "must-be-ignored", "extraData": map[string]any{"_kimiDeviceId": deviceID}})
	w = deviceRoute(&h, "POST", "kimi-coding", "poll", string(payload))
	if saved != nil || !strings.Contains(w.Body.String(), `"pending":true`) {
		t.Fatal(w.Body.String(), saved)
	}
	mode = "success"
	w = deviceRoute(&h, "POST", "kimi-coding", "poll", string(payload))
	if w.Code != 200 || saved["provider"] != "kimi" || saved["providerSpecificData"].(map[string]any)["deviceId"] != deviceID {
		t.Fatal(w.Code, saved)
	}
}
func TestKiloInitiationAndPoll(t *testing.T) {
	pollStatus := 202
	profiles := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/init":
			if r.Method != "POST" {
				t.Error(r.Method)
			}
			_, _ = w.Write([]byte(`{"code":"device","verificationUrl":"https://example.test/verify"}`))
		case "/poll/device":
			if r.Method != "GET" {
				t.Error("Kilo poll must GET")
			}
			w.WriteHeader(pollStatus)
			if pollStatus == 200 {
				_, _ = w.Write([]byte(`{"status":"approved","token":"kilo-access","userEmail":"op@example.test"}`))
			}
		case "/api/profile":
			profiles++
			if r.Header.Get("Authorization") != "Bearer kilo-access" {
				t.Error("profile bearer")
			}
			_, _ = w.Write([]byte(`{"organizations":[{"id":"org-1"}]}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	var saved map[string]any
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{InitiateURL: server.URL + "/init", PollURLBase: server.URL + "/poll", APIBaseURL: server.URL}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "kilocode", "device-code", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"interval":3`) {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, status := range []int{202, 403, 410, 500} {
		pollStatus = status
		w = deviceRoute(&h, "POST", "kilocode", "poll", `{"deviceCode":"device"}`)
		if w.Code != 200 || saved != nil || profiles != 0 {
			t.Fatal(status, w.Code, w.Body.String())
		}
	}
	pollStatus = 200
	w = deviceRoute(&h, "POST", "kilocode", "poll", `{"deviceCode":"device"}`)
	if w.Code != 200 || saved["accessToken"] != "kilo-access" || saved["refreshToken"] != nil || saved["email"] != "op@example.test" || profiles != 1 || saved["providerSpecificData"].(map[string]any)["orgId"] != "org-1" {
		t.Fatal(w.Code, saved)
	}
}

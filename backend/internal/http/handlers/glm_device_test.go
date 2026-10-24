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

func TestGLMCLIFlowAndPlanKey(t *testing.T) {
	state := "pending"
	pollToken := ""
	created := false
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/init":
			pollToken = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if len(pollToken) != 64 {
				t.Error("poll token entropy")
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"flow_id":"flow","authorize_url":"https://example.test/authorize","poll_interval_sec":3}}`))
		case "/poll/flow":
			if r.Header.Get("Authorization") != "Bearer "+pollToken {
				t.Error("wrong poll credential")
			}
			if state == "pending" {
				_, _ = w.Write([]byte(`{"code":0,"data":{"status":"pending"}}`))
			} else {
				_, _ = w.Write([]byte(`{"code":0,"data":{"status":"ready","token":"zcode-jwt","user":{"name":"Operator","email":"op@example.test","user_id":"u"},"zai":{"access_token":"zai-access","refresh_token":"zai-refresh"}}}`))
			}
		case "/login":
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload["token"] != "zai-access" {
				t.Error(payload)
			}
			_, _ = w.Write([]byte(`{"code":200,"data":{"access_token":"business"}}`))
		case "/api/biz/customer/getCustomerInfo":
			if r.Header.Get("Authorization") != "Bearer business" {
				t.Error("business bearer")
			}
			_, _ = w.Write([]byte(`{"code":0,"data":{"organizations":[{"organizationId":"wrong","organizationName":"Team","projects":[{"projectId":"team","projectType":"2"}]},{"organizationId":"org","organizationName":"default","projects":[{"projectId":"proj","projectName":"default","projectType":"1"}]}]}}`))
		case "/api/biz/v1/organization/org/projects/proj/api_keys":
			if r.Method == "GET" {
				_, _ = w.Write([]byte(`{"code":0,"data":[]}`))
			} else {
				created = true
				_, _ = w.Write([]byte(`{"code":0,"data":{"apiKey":"key-id"}}`))
			}
		case "/api/biz/v1/organization/org/projects/proj/api_keys/copy/key-id":
			_, _ = w.Write([]byte(`{"code":0,"data":{"secretKey":"secret-part"}}`))
		default:
			t.Error("unexpected URL", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig {
		return &providers.OAuthConfig{ProviderID: "zai", CLIInitURL: server.URL + "/init", CLIPollURL: server.URL + "/poll", BusinessLoginURL: server.URL + "/login", APIBaseURL: server.URL, PlanAPIKeyName: "zcode-api-key"}
	}, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	w := deviceRoute(&h, "GET", "glm", "device-code", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var device map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &device)
	if device["_zcodePollToken"] != pollToken {
		t.Fatal(device)
	}
	w = deviceRoute(&h, "POST", "glm", "poll", `{"deviceCode":"flow"}`)
	if w.Code != 200 || saved != nil || !strings.Contains(w.Body.String(), "Missing ZCode poll token") {
		t.Fatal(w.Code, w.Body.String())
	}
	payload, _ := json.Marshal(map[string]any{"deviceCode": "flow", "extraData": map[string]string{"_zcodePollToken": pollToken}})
	w = deviceRoute(&h, "POST", "glm", "poll", string(payload))
	if saved != nil || !strings.Contains(w.Body.String(), `"pending":true`) {
		t.Fatal(w.Body.String())
	}
	state = "ready"
	w = deviceRoute(&h, "POST", "glm", "poll", string(payload))
	if w.Code != 200 || !created || saved["accessToken"] != "key-id.secret-part" || saved["refreshToken"] != nil || strings.Contains(w.Body.String(), "secret-part") {
		t.Fatalf("ready: %d %s %v", w.Code, w.Body.String(), saved)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["zaiBusinessToken"] != "business" || specific["zaiRefreshToken"] != "zai-refresh" || specific["authMethod"] != "cli_poll" {
		t.Fatal(specific)
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestKiroAPIKeyRegionAndProfile(t *testing.T) {
	calls := 0
	status := 200
	profiles := `{"profiles":[{"arn":"arn:aws:codewhisperer:eu-west-1:account:profile/first"},{"profileArn":"arn:aws:codewhisperer:us-east-1:account:profile/match"}]}`
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.Header.Get("Content-Type") != "application/x-amz-json-1.0" || r.Header.Get("X-Amz-Target") != "AmazonCodeWhispererService.ListAvailableProfiles" || r.Header.Get("Authorization") != "Bearer dummy-key" {
			t.Error("profiles protocol")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["maxResults"] != float64(10) {
			t.Error(body)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(profiles))
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig { return &providers.OAuthConfig{ProfileURL: server.URL} }, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	call := func(body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.KiroAPIKey(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		return w
	}
	for _, body := range []string{`{}`, `{"apiKey":" "}`, `{"apiKey":1}`} {
		if w := call(body); w.Code != 400 || calls != 0 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, region := range []string{"localhost", "us-east-1.evil.test", "cn-north-1", "us-gov-west-1", "us-iso-east-1"} {
		payload, _ := json.Marshal(map[string]string{"apiKey": "dummy-key", "region": region})
		w := call(string(payload))
		if w.Code != 500 || calls != 0 || saved != nil || !strings.Contains(w.Body.String(), "API key validation failed") {
			t.Fatal(region, w.Code, w.Body.String())
		}
	}
	w := call(`{"apiKey":" dummy-key "}`)
	if w.Code != 200 || saved["authType"] != "api_key" || saved["accessToken"] != "dummy-key" || saved["refreshToken"] != nil || strings.Contains(w.Body.String(), "dummy-key") {
		t.Fatal(w.Code, saved)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["profileArn"] != "arn:aws:codewhisperer:us-east-1:account:profile/match" || specific["region"] != "us-east-1" {
		t.Fatal(specific)
	}
	expiry, err := time.Parse(time.RFC3339Nano, saved["expiresAt"].(string))
	if err != nil || time.Until(expiry) < 364*24*time.Hour {
		t.Fatal(saved)
	}
	saved = nil
	status = 401
	profiles = `secret upstream diagnostic`
	w = call(`{"apiKey":"dummy-key"}`)
	if w.Code != 500 || saved != nil || strings.Contains(w.Body.String(), "diagnostic") {
		t.Fatal(w.Code, w.Body.String())
	}
	status = 200
	profiles = `{"profiles":[]}`
	w = call(`{"apiKey":"dummy-key","region":"eu-west-1"}`)
	if w.Code != 200 || saved["providerSpecificData"].(map[string]any)["profileArn"] != nil {
		t.Fatal(w.Code, saved)
	}
}

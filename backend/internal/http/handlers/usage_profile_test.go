package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOllamaUsageFetchesProfileBestEffort(t *testing.T) {
	for _, profileStatus := range []int{200, 503} {
		calls := 0
		u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Header.Get("Authorization") != "Bearer key" {
				t.Fatal("wrong key")
			}
			if calls == 1 {
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"limits":{"session":{"usage":0.25}}}`))}, nil
			}
			if r.Method != "POST" || r.URL.Path != "/api/me" {
				t.Fatal(r.URL)
			}
			return &http.Response{StatusCode: profileStatus, Body: io.NopCloser(strings.NewReader(`{"Plan":"pro"}`))}, nil
		})}}
		w := httptest.NewRecorder()
		u.providerUsage(w, httptest.NewRequest("GET", "/", nil), map[string]any{"provider": "ollama", "apiKey": "key"})
		var result map[string]any
		json.Unmarshal(w.Body.Bytes(), &result)
		plan := "Pro"
		if profileStatus != 200 {
			plan = "Ollama Cloud"
		}
		if w.Code != 200 || calls != 2 || result["plan"] != plan || result["quotas"] == nil {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
}
func TestVercelUsageCredits(t *testing.T) {
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://ai-gateway.vercel.sh/v1/credits" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"balance":"2.50","total_used":"1.25"}`))}, nil
	})}}
	w := httptest.NewRecorder()
	u.providerUsage(w, httptest.NewRequest("GET", "/", nil), map[string]any{"provider": "vercel-ai-gateway", "apiKey": "key"})
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	quotas := result["quotas"].(map[string]any)
	remaining := quotas["Remaining (USD)"].(map[string]any)
	if remaining["remainingPercentage"] != float64(50) || remaining["remaining"] != 2.5 {
		t.Fatal(result)
	}
}

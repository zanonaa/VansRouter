package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCodexUsageNormalizesAllWindows(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(`{"plan_type":"plus","rate_limit":{"limit_reached":true,"primary_window":{"used_percent":125,"reset_at":1700000000},"secondary_window":{"percent_used":"25","reset_at":"1700000000000"}},"additional_rate_limits":[{"limit_name":"code_review","rate_limit":{"primary_window":{"used_percent":10}}},{"limit_name":"gpt-5.3-codex-spark","rate_limit":{"limit_reached":true,"secondary":{"used_percent":-1}}}],"rate_limit_reset_credits":{"available_count":"3"}}`), &data)
	out := normalizeCodexUsage(data)
	quotas := out["quotas"].(map[string]any)
	if out["plan"] != "plus" || out["limitReached"] != true || out["sparkLimitReached"] != true || len(quotas) != 4 {
		t.Fatal(out)
	}
	session := quotas["session"].(map[string]any)
	weekly := quotas["weekly"].(map[string]any)
	if session["used"] != float64(100) || weekly["used"] != float64(25) || session["resetAt"] != weekly["resetAt"] {
		t.Fatal(quotas)
	}
	if out["resetCredits"].(map[string]any)["availableCount"] != float64(3) {
		t.Fatal(out)
	}
}
func TestCodexUsageCallsRealEndpointAndHandlesUnavailable(t *testing.T) {
	for _, status := range []int{200, 429} {
		u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://chatgpt.com/backend-api/wham/usage" || r.Header.Get("Authorization") != "Bearer token" {
				t.Fatal(r.URL, r.Header)
			}
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"plan_type":"plus"}`))}, nil
		})}}
		w := httptest.NewRecorder()
		u.providerUsage(w, httptest.NewRequest("GET", "/", nil), map[string]any{"provider": "codex", "accessToken": "token"})
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		if status == 429 && !strings.Contains(w.Body.String(), "temporarily unavailable (429)") {
			t.Fatal(w.Body.String())
		}
	}
}

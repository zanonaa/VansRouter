package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKiroUsageFallsBackWithoutInjectingAPIKeyProfile(t *testing.T) {
	calls := 0
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("tokentype") != "API_KEY" {
			t.Fatal(r.Header)
		}
		if calls < 3 {
			return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(`denied`))}, nil
		}
		if r.Method != "GET" || r.URL.Host != "q.us-east-1.amazonaws.com" || r.URL.Query().Get("profileArn") != "" {
			t.Fatal(r.Method, r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"subscriptionInfo":{"subscriptionTitle":"Pro"},"usageBreakdownList":[{"resourceType":"Agent","currentUsageWithPrecision":1.5,"usageLimitWithPrecision":10}]}`))}, nil
	})}}
	result := u.kiroUsage(httptest.NewRequest("GET", "/", nil), "token", map[string]any{"providerSpecificData": map[string]any{"authMethod": "api_key"}})
	quota := result["quotas"].(map[string]any)["agent"].(map[string]any)
	if calls != 3 || result["plan"] != "Pro" || quota["remaining"] != float64(8.5) {
		t.Fatal(calls, result)
	}
}

func TestFreebuffUsageReadsSessionWithoutPost(t *testing.T) {
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/freebuff/session" || r.Header.Get("User-Agent") != "codebuff-cli/0.0.138" {
			t.Fatal(r.Method, r.URL, r.Header)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"accessTier":"limited","status":"active","model":"model","rateLimit":{"recentCount":1.3,"limit":5,"resetAt":"tomorrow"}}`))}, nil
	})}}
	result := u.freebuffUsage(httptest.NewRequest("GET", "/", nil), "token")
	encoded, _ := json.Marshal(result)
	if result["plan"] != "Freebuff (Limited)" || !strings.Contains(string(encoded), `"used":1.3`) || result["quotas"].(map[string]any)["model"].(map[string]any)["recurring"] != true {
		t.Fatal(string(encoded))
	}
	u.Client = &http.Client{Transport: kiroImportTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(`{"status":"country_blocked"}`))}, nil
	})}
	if result = u.freebuffUsage(httptest.NewRequest("GET", "/", nil), "token"); result["message"] != "Freebuff is not available in your region." {
		t.Fatal(result)
	}
}

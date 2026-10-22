package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenCodeUsageNormalizesQuotasAndEntitlement(t *testing.T) {
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://opencode.ai/zen/go/v1/usage" || r.Header.Get("Authorization") != "Bearer key" {
			t.Fatal(r.URL, r.Header)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"usage":{"rolling":{"percent":"125","resetsAt":1700000000},"weekly":{"percent":"bad"}}}`))}, nil
	})}}
	result := u.openCodeUsage(httptest.NewRequest("GET", "/", nil), "opencode-go", " key ")
	quota := result["quotas"].(map[string]any)["Rolling"].(map[string]any)
	if result["plan"] != "OpenCode Go" || quota["used"] != float64(100) || result["quotas"].(map[string]any)["Weekly"] != nil {
		t.Fatal(result)
	}
	u.Client = &http.Client{Transport: kiroImportTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(`{"error":{"type":"EntitlementError"}}`))}, nil
	})}
	result = u.openCodeUsage(httptest.NewRequest("GET", "/", nil), "opencode-zen", "key")
	if result["message"] != "OpenCode Zen billing required for this API key." {
		t.Fatal(result)
	}
}

func TestGitHubAndQoderUsage(t *testing.T) {
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "token access" || r.Header.Get("X-GitHub-Api-Version") != "2025-04-01" {
			t.Fatal(r.Header)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"copilot_plan":"pro","quota_reset_date":"2026-11-01T00:00:00Z","quota_snapshots":{"chat":{"entitlement":100,"remaining":25,"unlimited":false}}}`))}, nil
	})}}
	result, status, err := u.githubUsage(httptest.NewRequest("GET", "/", nil), "access")
	quota := result["quotas"].(map[string]any)["chat"].(map[string]any)
	if err != nil || status != 200 || quota["used"] != float64(75) || quota["remaining"] != float64(25) {
		t.Fatal(result, status, err)
	}
	_, _, err = u.githubUsage(httptest.NewRequest("GET", "/", nil), "")
	if err == nil || !strings.Contains(err.Error(), "No GitHub access token") {
		t.Fatal(err)
	}
	u.Client = &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://openapi.qoder.sh/api/v2/quota/usage" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"userQuota":{"total":"10","used":4},"expiresAt":1700000000000,"isQuotaExceeded":1}`))}, nil
	})}
	qoder := u.qoderUsage(httptest.NewRequest("GET", "/", nil), "token")
	encoded, _ := json.Marshal(qoder)
	if qoder["isQuotaExceeded"] != true || !strings.Contains(string(encoded), `"used":4`) || qoder["expiresAt"] != float64(1700000000000) {
		t.Fatal(string(encoded))
	}
}

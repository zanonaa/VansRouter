package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaudeUsageNormalizesScopedWindowsAndGrants(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(`{"five_hour":{"utilization":87,"resets_at":1700000000},"seven_day_sonnet":{"utilization":12},"limits":[{"kind":"weekly_scoped","scope":{"model":{"display_name":" OPUS "}},"percent":140}],"extra_usage":{"enabled":true},"cedar_ember":{"eligible":true,"next_grant_id":"second","grants":[{"id":"first","resets_left":2},{"id":"paused","resets_left":5,"paused":true},{"id":"second","resets_left":1,"clears":["five_hour"]}]}}`), &data)
	out := normalizeClaudeUsage(data)
	quotas := out["quotas"].(map[string]any)
	if len(quotas) != 3 || quotas["session (5h)"].(map[string]any)["remaining"] != float64(13) || quotas["weekly opus (7d)"].(map[string]any)["used"] != float64(100) {
		t.Fatal(out)
	}
	credits := out["resetCredits"].(map[string]any)
	if credits["availableCount"] != float64(3) || credits["nextGrantId"] != "second" || len(credits["grants"].([]any)) != 3 {
		t.Fatal(credits)
	}
}
func TestClaudeUsageFallsBackToOrganization(t *testing.T) {
	calls := 0
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		status := 200
		body := ""
		switch calls {
		case 1:
			if r.URL.Query().Get("cedar_ember") != "1" || !strings.Contains(r.Header.Get("User-Agent"), "external, cli") {
				t.Fatal(r.URL, r.Header)
			}
			status = 429
			body = `{}`
		case 2:
			if r.URL.Path != "/v1/settings" {
				t.Fatal(r.URL)
			}
			body = `{"organization_id":"org","organization_name":"Org","plan":"Pro"}`
		case 3:
			if r.URL.Path != "/v1/organizations/org/usage" {
				t.Fatal(r.URL)
			}
			body = `{"quota":5}`
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	out := u.claudeUsage(httptest.NewRequest("GET", "/", nil), "token")
	if calls != 3 || out["plan"] != "Pro" || out["quotas"] == nil {
		t.Fatal(out, calls)
	}
}

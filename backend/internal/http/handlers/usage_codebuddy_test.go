package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCodeBuddySeparatesRefillAndBonus(t *testing.T) {
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.Host != "copilot.tencent.com" || r.Header.Get("X-IDE-Type") != "CLI" || r.Header.Get("Authorization") != "Bearer token" {
			t.Fatal(r.Method, r.URL, r.Header)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":0,"data":{"Response":{"Data":{"Accounts":[{"PackageName":"Pro","CycleStartTime":"2026-10-01T00:00:00Z","CycleEndTime":"2026-11-01T00:00:00Z","DeductionEndTime":"2027-01-01T00:00:00Z","CycleCapacityUsedPrecise":"6.5","CycleCapacitySize":500},{"CycleStartTime":"2026-10-01T00:00:00Z","CycleEndTime":"2026-10-31T00:00:00Z","DeductionEndTime":"2026-10-31T00:00:00Z","CapacityUsed":2,"CapacitySizePrecise":"10"}]}}}}`))}, nil
	})}}
	result := u.codeBuddyUsage(httptest.NewRequest("GET", "/", nil), "codebuddy-cn", map[string]any{"accessToken": "token"})
	monthly := result["quotas"].(map[string]any)["Monthly"].(map[string]any)
	bonus := result["quotas"].(map[string]any)["Bonus Pack 1"].(map[string]any)
	if result["plan"] != "Pro" || monthly["used"] != 6.5 || monthly["recurring"] != true || bonus["total"] != float64(10) || bonus["recurring"] != false {
		t.Fatal(result)
	}
	if result = parseCodeBuddyUsage(map[string]any{"code": float64(7), "msg": "bad"}, "codebuddy-intl"); !strings.Contains(ttsString(result["message"]), "CodeBuddy Intl quota error: bad") {
		t.Fatal(result)
	}
}

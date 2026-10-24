package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAntigravityOnboardingRetriesAndStops(t *testing.T) {
	calls := 0
	waits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer dummy" || r.Header.Get("User-Agent") != "ag-test" || r.Header.Get("x-request-source") != "local" {
			t.Error("onboarding headers")
		}
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["tierId"] != "tier-9" || payload["metadata"].(map[string]any)["platform"] != float64(oauthPlatform()) {
			t.Error(payload)
		}
		_ = json.NewEncoder(w).Encode(map[string]bool{"done": calls == 3})
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), RetryWait: func(context.Context, time.Duration) error { waits++; return nil }}
	h.antigravityOnboard("dummy", "tier-9", server.URL, "ag-test")
	if calls != 3 || waits != 2 {
		t.Fatal(calls, waits)
	}
}

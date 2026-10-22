package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestClaudeUsageCacheDeduplicatesAndForceRefreshes(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"five_hour":{"utilization":25}}`))}, nil
	})}}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := u.claudeUsage(httptest.NewRequest("GET", "/", nil), "token")
			if result["quotas"] == nil {
				t.Error(result)
			}
		}()
	}
	<-started
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("calls %d", calls.Load())
	}
	result := u.claudeUsage(httptest.NewRequest("GET", "/", nil), "token")
	result["quotas"].(map[string]any)["session (5h)"] = nil
	next := u.claudeUsage(httptest.NewRequest("GET", "/", nil), "token")
	if next["quotas"].(map[string]any)["session (5h)"] == nil {
		t.Fatal("cache corrupted by caller")
	}
	u.claudeUsage(httptest.NewRequest("GET", "/?force=true", nil), "token")
	if calls.Load() != 2 {
		t.Fatal("force did not bypass cache")
	}
}
func TestClaudeUsage429SkipsOAuthDuringCooldown(t *testing.T) {
	oauth, legacy := 0, 0
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		status := 403
		if strings.Contains(r.URL.Path, "oauth/usage") {
			oauth++
			status = 429
		} else {
			legacy++
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}}
	for i := 0; i < 2; i++ {
		u.claudeUsage(httptest.NewRequest("GET", "/", nil), "token")
	}
	if oauth != 1 || legacy != 2 {
		t.Fatalf("oauth %d legacy %d", oauth, legacy)
	}
}

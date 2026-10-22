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

func TestGeminiUsageResolvesProjectAndBuckets(t *testing.T) {
	calls := 0
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{"cloudaicompanionProject":{"id":" project "},"currentTier":{"name":"Pro"}}`
		if calls == 2 {
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), `"project":"project"`) {
				t.Fatal(string(raw))
			}
			body = `{"buckets":[{"modelId":"gemini","remainingFraction":0.25,"resetTime":"2026-11-01T00:00:00Z"},{"remainingFraction":1}]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	result := u.geminiUsage(httptest.NewRequest("GET", "/", nil), "token", map[string]any{})
	quota := result["quotas"].(map[string]any)["gemini"].(map[string]any)
	if result["plan"] != "Pro" || quota["used"] != float64(750) || len(result["quotas"].(map[string]any)) != 1 {
		t.Fatal(result)
	}
}

func TestAntigravityFreeTierSkipsModelsButKeepsWeekly(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		body := `{"paidTier":{"id":"free-tier"},"currentTier":{"name":"Free"}}`
		switch {
		case strings.Contains(r.URL.Path, "fetchAvailableModels"):
			body = `{"models":{"gemini-3-flash":{"quotaInfo":{"remainingFraction":1}},"internal":{"isInternal":true,"quotaInfo":{"remainingFraction":1}}}}`
		case strings.Contains(r.URL.Path, "retrieveUserQuotaSummary"):
			if calls.Add(1) == 1 {
				close(started)
				<-release
			}
			body = `{"groups":[{"displayName":"Gemini","buckets":[{"bucketId":"weekly","remainingFraction":0.5,"disabled":false},{"displayName":"weekly backup","remainingFraction":0}]}]}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := u.antigravityUsage(httptest.NewRequest("GET", "/", nil), "token")
			if result["quotas"].(map[string]any)["gemini-3-flash"] != nil || result["quotas"].(map[string]any)["gemini_weekly"].(map[string]any)["used"] != float64(500) {
				t.Error(result)
			}
		}()
	}
	<-started
	close(release)
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("weekly calls %d", calls.Load())
	}
}

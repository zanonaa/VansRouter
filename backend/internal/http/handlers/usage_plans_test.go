package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGLMAndMiniMaxUsage(t *testing.T) {
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "open.bigmodel.cn" {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":{"level":"pro","limits":[{"type":"TOKENS_LIMIT","unit":3,"number":5,"percentage":25,"nextResetTime":1700000000000}]}}`))}, nil
	})}}
	result := u.glmUsage(httptest.NewRequest("GET", "/", nil), "glm-cn", "key")
	if result["plan"] != "Pro" || result["quotas"].(map[string]any)["Session (5h)"].(map[string]any)["remaining"] != float64(75) {
		t.Fatal(result)
	}
	calls := 0
	u.Client = &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}
		if !strings.Contains(r.URL.Path, "coding_plan/remains") {
			t.Fatal(r.URL)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"base_resp":{"status_code":0},"model_remains":[{"model_name":"general","current_interval_remaining_percent":25}]}`))}, nil
	})}
	result = u.miniMaxUsage(httptest.NewRequest("GET", "/", nil), "minimax", "key")
	quota := result["quotas"].(map[string]any)["M-series (5h)"].(map[string]any)
	if calls != 2 || quota["remainingPercentage"] != float64(25) || quota["used"] != float64(75) {
		t.Fatal(calls, result)
	}
}

func TestKimiUsageAPIKeyAndPermission(t *testing.T) {
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("x-api-key") != "key" || r.Header.Get("Authorization") != "" {
			t.Fatal(r.Header)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"usage":{"limit":100,"used":40,"remaining":60},"user":{"membership":{"level":"LEVEL_ADVANCED"}}}`))}, nil
	})}}
	result := u.kimiUsage(httptest.NewRequest("GET", "/", nil), map[string]any{"apiKey": "key", "accessToken": "token"})
	if result["plan"] != "Allegro" || result["quotas"].(map[string]any)["Weekly"].(map[string]any)["remainingPercentage"] != float64(60) {
		t.Fatal(result)
	}
	u.Client = &http.Client{Transport: kiroImportTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 403, Body: io.NopCloser(strings.NewReader(`{"reason":"REASON_FEATURE_NO_PERMISSION","message":"subscribe first"}`))}, nil
	})}
	result = u.kimiUsage(httptest.NewRequest("GET", "/", nil), map[string]any{"accessToken": "token"})
	if result["message"] != "subscribe first" {
		t.Fatal(result)
	}
}

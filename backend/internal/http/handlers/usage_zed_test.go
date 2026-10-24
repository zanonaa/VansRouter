package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestZedUsageAuthAndTokenBilling(t *testing.T) {
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "user token" || r.Header.Get("x-zed-system-id") != "system" {
			t.Fatal(r.Header)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"plan":{"plan_v3":"zed_pro","subscription_period":{"ended_at":"2026-11-01T00:00:00Z"},"usage":{"edit_predictions":{"used":20,"limit":100},"model_requests":{"used":4,"limit":{"limited":0}}},"has_overdue_invoices":true}}`))}, nil
	})}}
	result := u.zedUsage(httptest.NewRequest("GET", "/", nil), map[string]any{"accessToken": " token ", "providerSpecificData": map[string]any{"userId": "user", "systemId": "system"}})
	predictions := result["quotas"].(map[string]any)["Edit Predictions"].(map[string]any)
	if result["plan"] != "Zed Pro" || predictions["remainingPercentage"] != float64(80) || result["quotas"].(map[string]any)["Hosted Model Requests"] != nil || result["hasOverdueInvoices"] != true {
		t.Fatal(result)
	}
	if result = u.zedUsage(httptest.NewRequest("GET", "/", nil), map[string]any{"accessToken": "token"}); result["message"] != "Zed credential is missing user id. Re-connect Zed to view quota." {
		t.Fatal(result)
	}
}

package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type usageRefreshStore struct {
	connection map[string]any
	updated    map[string]any
}

func (s *usageRefreshStore) Get(context.Context, string) (map[string]any, error) {
	return s.connection, nil
}
func (s *usageRefreshStore) Update(_ context.Context, _ string, patch map[string]any) (map[string]any, error) {
	s.updated = patch
	return s.connection, nil
}

// O access token do Grok CLI expira em 6h. A tela de usage deve renovar com o
// refresh token (como a origem) em vez de exibir "authentication expired".
func TestGrokUsageRefreshesExpiredToken(t *testing.T) {
	billingCalls := 0
	u := &UsageContracts{Connections: &usageRefreshStore{connection: map[string]any{"id": "conn"}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == "auth.x.ai" {
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "stale" || r.Form.Get("client_id") != "b1a00492-073a-47ea-816f-4c329264a828" {
				t.Fatalf("form de refresh inesperado: %v", r.Form)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"fresh","refresh_token":"rotated","expires_in":21600}`))}, nil
		}
		auth := r.Header.Get("Authorization")
		if strings.Contains(r.URL.Path, "/billing") {
			billingCalls++
			if auth != "Bearer fresh" {
				return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader(`{"error":"expired"}`))}, nil
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"config":{"monthlyLimit":{"val":100},"includedUsed":{"val":100},"billingPeriodEnd":"2026-10-28T00:00:00Z"}}`))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"subscriptionTier":"super_grok"}`))}, nil
	})}}
	store := u.Connections.(*usageRefreshStore)
	store.connection["accessToken"] = "stale"
	store.connection["refreshToken"] = "stale"
	store.connection["providerSpecificData"] = map[string]any{"email": "user@example.test"}

	result := u.grokUsage(httptest.NewRequest("GET", "/", nil), store.connection)
	if message := ttsString(result["message"]); strings.Contains(message, "authentication expired") {
		t.Fatalf("não renovou: %s", message)
	}
	if store.updated["accessToken"] != "fresh" || store.updated["refreshToken"] != "rotated" {
		t.Fatalf("patch não persistido: %v", store.updated)
	}
	if billingCalls != 2 {
		t.Fatalf("billing chamado %d vezes", billingCalls)
	}
	if quotas, _ := result["quotas"].(map[string]any); quotas["Monthly included"] == nil {
		t.Fatalf("quotas perdidas: %v", result)
	}
}

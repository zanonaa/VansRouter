package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type resetConnectionFixture struct{ connection map[string]any }

func (s resetConnectionFixture) Get(context.Context, string) (map[string]any, error) {
	return s.connection, nil
}
func (s resetConnectionFixture) Update(context.Context, string, map[string]any) (map[string]any, error) {
	return s.connection, nil
}

func TestCodexCreditsConsumeUsesServerRedeemID(t *testing.T) {
	fixture := resetConnectionFixture{map[string]any{"provider": "codex", "authType": "access_token", "accessToken": "secret"}}
	var redeemID string
	u := &UsageContracts{Connections: fixture, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/backend-api/wham/rate-limit-reset-credits/consume" {
			t.Fatalf("wrong consume endpoint: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		redeemID = ttsString(payload["redeem_request_id"])
		if redeemID == "" || redeemID == "client-id" {
			t.Fatal("missing server-generated id")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":"reset","windows_reset":1,"credit":{"status":"consumed"}}`))}, nil
	})}}
	w := httptest.NewRecorder()
	u.CodexCredits(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"redeem_request_id":"client-id"}`)))
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["redeemRequestId"] != redeemID || result["reset"] != true {
		t.Fatal("wrong consume result")
	}
}

func TestClaudeResetResolvesOrganization(t *testing.T) {
	calls := 0
	u := &UsageContracts{Connections: resetConnectionFixture{map[string]any{"provider": "claude", "authType": "oauth", "accessToken": "secret"}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			if r.Method != "GET" || r.URL.Path != "/api/oauth/profile" {
				t.Fatal("missing profile resolution")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"organization":{"uuid":"org-id"}}`))}, nil
		}
		if r.URL.Path != "/api/organizations/org-id/reset_rate_limits" {
			t.Fatal("wrong reset URL")
		}
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["program"] != "cedar_ember" || payload["grant_id"] != "grant_1" || !nodeTruthy(payload["request_id"]) {
			t.Fatal("wrong reset payload")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"result":"reset","resets_left":2}`))}, nil
	})}}
	w := httptest.NewRecorder()
	u.ClaudeReset(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"grantId":"grant_1"}`)))
	if w.Code != 200 || calls != 2 {
		t.Fatalf("status %d calls %d", w.Code, calls)
	}
	w = httptest.NewRecorder()
	u.ClaudeReset(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"grantId":"invalid/"}`)))
	if w.Code != 500 || calls != 2 {
		t.Fatal("invalid grant sent upstream")
	}
}

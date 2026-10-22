package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestKiroSocialPollClassification(t *testing.T) {
	for _, test := range []struct {
		body         string
		status, want int
		needle       string
	}{
		{`{"error":"authorization_pending"}`, 400, 200, `"pending":true`},
		{`{"error":"slow_down"}`, 429, 200, `"pending":true`},
		{`{"error":"access_denied"}`, 403, 403, `"error":"access_denied"`},
		{`{}`, 200, 502, `"error":"invalid_token_response"`},
	} {
		h := &OAuthEngine{Client: &http.Client{Transport: kiroImportTransport(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: test.status, Body: io.NopCloser(strings.NewReader(test.body))}, nil
		})}}
		w := httptest.NewRecorder()
		h.KiroSocialExchange(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"deviceCode":"device","provider":"google"}`)))
		if w.Code != test.want || !strings.Contains(w.Body.String(), test.needle) {
			t.Fatalf("poll status %d body %s", w.Code, w.Body.String())
		}
	}
}

func TestKiroSocialIdentityDoesNotMergeProfileOnly(t *testing.T) {
	existing := []map[string]any{{"id": "a", "authType": "oauth", "email": "one@example.invalid", "providerSpecificData": map[string]any{"profileArn": "profile"}}}
	if findKiroSocialIdentity(existing, "profile", "") != nil {
		t.Fatal("profile-only merged accounts")
	}
	if findKiroSocialIdentity(existing, "profile", "two@example.invalid") != nil {
		t.Fatal("conflicting emails merged")
	}
	if findKiroSocialIdentity(existing, "profile", "ONE@example.invalid") == nil {
		t.Fatal("matching identity not reused")
	}
}

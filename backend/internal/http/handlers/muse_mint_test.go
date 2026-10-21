package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestMuseMintErrorsAndRedirect(t *testing.T) {
	for _, tt := range []struct {
		body, want string
		status     int
	}{
		{`{"require_payment":true,"action_url":"https://example.test/activate"}`, "subscription required: https://example.test/activate", 200},
		{`{}`, "missing api_key", 200}, {`invalid`, "invalid JSON", 200},
		{`{"title":"Denied","detail":"Subscription","action_url":"https://example.test/fix"}`, "Denied: Subscription — https://example.test/fix", 403},
	} {
		t.Run(tt.want, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()
			h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig { return &providers.OAuthConfig{MuseKeyURL: server.URL} }}
			_, err := h.museMint(context.Background(), "dummy")
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatal(err)
			}
		})
	}
	followed := false
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { followed = true }))
	defer destination.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, 302) }))
	defer redirect.Close()
	h := OAuthEngine{Client: redirect.Client(), Config: func(string) *providers.OAuthConfig { return &providers.OAuthConfig{MuseKeyURL: redirect.URL} }}
	if _, err := h.museMint(context.Background(), "dummy"); err == nil || followed {
		t.Fatal("mint followed redirect", err, followed)
	}
}

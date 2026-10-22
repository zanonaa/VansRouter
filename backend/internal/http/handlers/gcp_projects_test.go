package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type gcpStore struct {
	item  map[string]any
	patch map[string]any
}

func (s *gcpStore) Get(context.Context, string) (map[string]any, error) { return s.item, nil }
func (s *gcpStore) Update(_ context.Context, _ string, patch map[string]any) (map[string]any, error) {
	s.patch = patch
	return patch, nil
}
func TestGCPProjectsRefreshAndMapping(t *testing.T) {
	store := &gcpStore{item: map[string]any{"id": "c", "provider": "gemini-cli", "accessToken": "old", "refreshToken": "refresh", "expiresAt": time.Now().Add(-time.Hour).Format(time.RFC3339)}}
	calls := 0
	handler := &GCPProjects{Connections: store, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			config := providers.GetOAuthConfig("gemini-cli")
			if r.URL.String() != config.TokenURL || r.Method != "POST" {
				t.Fatal("wrong refresh request")
			}
			r.ParseForm()
			if r.Form.Get("refresh_token") != "refresh" {
				t.Fatal("missing refresh token")
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"new","expires_in":3600}`))}, nil
		}
		if r.Header.Get("Authorization") != "Bearer new" || r.URL.Host != "cloudresourcemanager.googleapis.com" {
			t.Fatal("projects used wrong token or target")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"projects":[{"projectId":"project","name":"Name","secret":"not-returned"}]}`))}, nil
	})}}
	w := httptest.NewRecorder()
	handler.Get(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || calls != 2 || store.patch["accessToken"] != "new" || strings.Contains(w.Body.String(), "not-returned") {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}
func TestGCPProjectsWrongProviderAndMissing(t *testing.T) {
	store := &gcpStore{}
	handler := &GCPProjects{Connections: store}
	w := httptest.NewRecorder()
	handler.Get(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	store.item = map[string]any{"provider": "openai"}
	w = httptest.NewRecorder()
	handler.Get(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

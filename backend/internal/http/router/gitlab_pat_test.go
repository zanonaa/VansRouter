package router

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestGitlabPATRouteAndGuard(t *testing.T) {
	authentication, _ := wiredAuth(t)
	r := NewWithNodes(nil, authentication, nil, &handlers.Providers{OAuth: &handlers.OAuthEngine{}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/gitlab/pat", strings.NewReader(`{}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Personal Access Token is required") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/api/oauth/gitlab/pat", nil))
	if w.Code != 204 || w.Header().Get("Allow") != "OPTIONS, POST" {
		t.Fatal(w.Code, w.Header())
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/gitlab/pat", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatal("unguarded PAT", w.Code)
	}
}

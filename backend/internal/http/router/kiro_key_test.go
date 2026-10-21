package router

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestKiroKeyRouteGuard(t *testing.T) {
	auth, _ := wiredAuth(t)
	r := NewWithNodes(nil, auth, nil, &handlers.Providers{OAuth: &handlers.OAuthEngine{}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/kiro/api-key", strings.NewReader(`{}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "API key is required") {
		t.Fatal(w.Code, w.Body.String())
	}
	auth.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/kiro/api-key", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatal("unguarded")
	}
}

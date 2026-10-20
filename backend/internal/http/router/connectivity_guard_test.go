package router

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestConnectivityRoutesRemainProtected(t *testing.T) {
	authentication, _ := wiredAuth(t)
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	api := &handlers.Providers{Pools: &handlers.ProxyPools{}}
	r := NewWithNodes(nil, authentication, &handlers.Nodes{}, api)
	for _, path := range []string{"/api/provider-nodes/validate", "/api/proxy-pools/pool/test"} {
		for _, method := range []string{"POST", "OPTIONS"} {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(`{}`)))
			if w.Code != 401 {
				t.Fatalf("%s %s: %d", method, path, w.Code)
			}
		}
	}
	// Com requireLogin=false, validator responde erro de entrada e não acessa rede.
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": false}, nil }
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/provider-nodes/validate", strings.NewReader(`{}`)))
	if w.Code != 400 {
		t.Fatalf("validator route not wired: %d %s", w.Code, w.Body.String())
	}
}

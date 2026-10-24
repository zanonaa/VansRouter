package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

type probePoolStore struct {
	differentialPoolStore
	pool  map[string]any
	patch map[string]any
}

func (s *probePoolStore) Get(context.Context, string) (map[string]any, error) { return s.pool, nil }
func (s *probePoolStore) Update(_ context.Context, _ string, patch map[string]any) (map[string]any, error) {
	s.patch = patch
	return patch, nil
}
func TestPoolProbePersistsResult(t *testing.T) {
	for _, status := range []int{200, 503} {
		relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
		store := &probePoolStore{pool: map[string]any{"proxyUrl": relay.URL, "type": "vercel"}}
		h := ProxyPools{Store: store}
		r := httptest.NewRequest("POST", "/api/proxy-pools/p/test", nil)
		route := chi.NewRouteContext()
		route.URLParams.Add("id", "p")
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
		w := httptest.NewRecorder()
		h.Test(ProxyProbe{})(w, r)
		relay.Close()
		if w.Code != 200 || store.patch["isActive"] != (status == 200) || store.patch["lastTestedAt"] == nil {
			t.Fatalf("result: %d %s %v", w.Code, w.Body.String(), store.patch)
		}
		if status == 503 && store.patch["lastError"] != "Proxy test failed with status 503" {
			t.Fatal(store.patch)
		}
		if status == 200 && store.patch["lastError"] != nil {
			t.Fatal(store.patch)
		}
		store.pool = nil
		w = httptest.NewRecorder()
		h.Test(ProxyProbe{})(w, r)
		if w.Code != 404 || !strings.Contains(w.Body.String(), "Proxy pool not found") {
			t.Fatal(w.Body.String())
		}
	}
}

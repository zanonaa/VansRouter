package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestProxyTestSettings(t *testing.T) {
	authentication, _ := wiredAuth(t)
	r := NewWithNodes(nil, authentication, nil, &handlers.Providers{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/settings/proxy-test", strings.NewReader(`{}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "proxyUrl is required") {
		t.Fatalf("empty: %d %s", w.Code, w.Body.String())
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer upstream.Close()
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer target.Close()
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/settings/proxy-test", strings.NewReader(`{"proxyUrl":"`+upstream.URL+`","testUrl":"`+target.URL+`/check"}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"ok":true`) {
		t.Fatalf("ok: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/settings/proxy-test", strings.NewReader(`{"proxyUrl":"http://127.0.0.1:1","testUrl":"`+target.URL+`"}`)))
	if w.Code == 200 {
		t.Fatal("bad proxy accepted")
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/settings/proxy-test", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatal("unguarded")
	}
}

package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHelloWorld(t *testing.T) {
	response := httptest.NewRecorder()
	New().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || response.Body.String() != "Hello World\n" {
		t.Fatalf("resposta: %d %q", response.Code, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Fatal("tipo de conteúdo incorreto")
	}
}

func TestBootstrapRoutes(t *testing.T) {
	router := New(func(context.Context) error { return nil })
	for _, tt := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/api/health", 200}, {"GET", "/api/ready", 200}, {"GET", "/api/init", 200},
		{"HEAD", "/api/health", 200}, {"HEAD", "/api/ready", 200}, {"HEAD", "/api/init", 200},
		{"OPTIONS", "/api/health", 204}, {"OPTIONS", "/api/ready", 204}, {"OPTIONS", "/api/init", 204}, {"OPTIONS", "/api/locale", 204},
		{"GET", "/api/locale", 405}, {"POST", "/api/health", 405},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, nil))
		if w.Code != tt.status {
			t.Errorf("%s %s: %d, esperado %d", tt.method, tt.path, w.Code, tt.status)
		}
		if (tt.method == "HEAD" || tt.method == "OPTIONS") && w.Body.Len() != 0 {
			t.Errorf("%s %s retornou corpo", tt.method, tt.path)
		}
	}
}

func TestAPIIsNotImplementedYet(t *testing.T) {
	response := httptest.NewRecorder()
	New().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("esperado 404, recebido %d", response.Code)
	}
}

package router

import (
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

func TestAPIIsNotImplementedYet(t *testing.T) {
	response := httptest.NewRecorder()
	New().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("esperado 404, recebido %d", response.Code)
	}
}

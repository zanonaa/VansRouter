package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func spaFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write := func(rel, content string) {
		target := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<!doctype html><html><body><div id=\"root\"></div></body></html>")
	write("assets/app-abc123.js", "console.log('app')")
	write("assets/style-xyz.css", "body{}")
	write("favicon.svg", "<svg/>")
	write("i18n/pt.json", "{}")
	return dir
}

func spaTestHandler(t *testing.T) http.Handler {
	t.Helper()
	return NewWithSPA(func(_ context.Context) error { return nil }, nil, nil, nil, spaFixture(t))
}

func TestSPASameOriginServesIndexAssetsAnd404(t *testing.T) {
	handler := spaTestHandler(t)

	// Rotas da SPA servem o index (fallback) sem cache.
	for _, target := range []string{"/", "/dashboard", "/dashboard/providers/openai/x", "/masuk", "/login", "/callback", "/landing"} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "<div id=\"root\">") {
			t.Fatalf("%s -> %d %s", target, w.Code, w.Body.String())
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s cache %q", target, w.Header().Get("Cache-Control"))
		}
	}

	// Assets públicos do build: servidos com cache imutável e tipo correto.
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/app-abc123.js", nil))
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" || !strings.Contains(w.Header().Get("Content-Type"), "text/javascript") {
		t.Fatalf("asset js: %d %v %s", w.Code, w.Header(), w.Body.String())
	}

	// Público sem hash (logo/favicon/ícones): troca entre builds precisa
	// aparecer no próximo load — no-cache revalida por ETag/Last-Modified.
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/favicon.svg", nil))
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-cache" || !strings.Contains(w.Header().Get("Content-Type"), "svg") {
		t.Fatalf("favicon: %d %v", w.Code, w.Header())
	}

	// API inexistente: 404 sem corpo HTML da SPA.
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/rota-que-nao-existe", nil))
	if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "<div id=\"root\">") {
		t.Fatalf("api 404: %d %s", w.Code, w.Body.String())
	}

	// Fora da SPA e fora do build: 404, não index.
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/rota-aleatoria", nil))
	if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "<div id=\"root\">") {
		t.Fatalf("fora da spa: %d %s", w.Code, w.Body.String())
	}

	// Path traversal não escapa do diretório do build.
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/assets/../../etc/passwd", nil))
	if w.Code != http.StatusOK {
		t.Logf("traversal bloqueado com status %d", w.Code)
	} else if strings.Contains(w.Body.String(), "root:") {
		t.Fatal("traversal expôs /etc/passwd")
	}

	// HEAD da rota da SPA funciona sem corpo.
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/dashboard", nil))
	if w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Fatalf("head: %d %q", w.Code, w.Body.String())
	}
}

func TestSPADisabledKeepsHelloWorld(t *testing.T) {
	handler := NewWithSPA(func(_ context.Context) error { return nil }, nil, nil, nil, "")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Hello World") {
		t.Fatalf("sem SPA_DIR esperado Hello World: %d %q", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("sem SPA_DIR /dashboard deve ser 404, obtido %d", w.Code)
	}
}

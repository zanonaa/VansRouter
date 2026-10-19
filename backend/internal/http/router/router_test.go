package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/auth"
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

// wiredAuth monta um autenticador completo com gravação em memória.
func wiredAuth(t *testing.T) (*auth.HTTP, *map[string]bool) {
	t.Helper()
	sessions, err := auth.LoadSessions(t.TempDir(), "segredo-ficticio-para-testes")
	if err != nil {
		t.Fatal(err)
	}
	written := map[string]bool{}
	authentication := &auth.HTTP{
		Sessions: sessions,
		Limiter:  auth.NewLimiter(),
		Settings: func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": false}, nil },
		ChangeP:  func(context.Context, string) (map[string]any, error) { written["called"] = true; return nil, nil },
	}
	return authentication, &written
}

func TestSettingsRoutesRequireAuth(t *testing.T) {
	authentication, _ := wiredAuth(t)
	// requireLogin=true: leitura exige sessão; aqui sem cookie → 401.
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	router := NewWithAuth(func(context.Context) error { return nil }, authentication)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/settings", nil))
	if w.Code != http.StatusUnauthorized || !strings.Contains(w.Body.String(), "Unauthorized") {
		t.Fatalf("settings sem sessão: %d %s", w.Code, w.Body.String())
	}
	// requireLogin=false libera a leitura.
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": false}, nil }
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodHead, "/api/settings", nil))
	if w.Code != http.StatusOK || w.Body.Len() != 0 {
		t.Fatalf("settings HEAD: %d", w.Code)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodOptions, "/api/settings", nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("settings OPTIONS: %d", w.Code)
	}
}

func TestResetPasswordLocalOnlyGate(t *testing.T) {
	authentication, _ := wiredAuth(t)
	router := NewWithAuth(func(context.Context) error { return nil }, authentication)
	request := func(remote string) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", nil)
		req.RemoteAddr = remote
		return req
	}
	// requireLogin=true: só sessão local autenticada passa.
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w := httptest.NewRecorder()
	router.ServeHTTP(w, request("192.0.2.10:1234"))
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "Local only: CLI token required") {
		t.Fatalf("reset remoto: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, request("127.0.0.1:1234"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("reset local sem sessão: %d", w.Code)
	}
	// requireLogin=false: local passa e o handler executa (corpo vazio → 400); remoto continua bloqueado.
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": false}, nil }
	w = httptest.NewRecorder()
	router.ServeHTTP(w, request("127.0.0.1:1234"))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("reset local com requireLogin=false deveria executar handler com corpo vazio (400): %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, request("192.0.2.10:1234"))
	if w.Code != http.StatusForbidden {
		t.Fatalf("remoto passou com requireLogin=false: %d", w.Code)
	}
	// OPTIONS local-only também passa pelo guard.
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodOptions, "/api/auth/reset-password", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("reset OPTIONS remoto deveria ser bloqueado: %d", w.Code)
	}
}

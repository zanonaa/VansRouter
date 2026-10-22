package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
	"strings"
	"testing"
)

func TestVersionPublicSkip(t *testing.T) {
	t.Setenv("VANSROUTER_SKIP_UPDATE_CHECK", "1")
	auth, _ := wiredAuth(t)
	r := NewWithAuth(nil, auth)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/version", nil))
	var value map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &value)
	if w.Code != 200 || value["currentVersion"] != "1.2.0" || value["latestVersion"] != nil || value["hasUpdate"] != false {
		t.Fatal(w.Code, value)
	}
	for _, method := range []string{"HEAD", "OPTIONS"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/version", nil))
		if w.Body.Len() != 0 || (w.Code != 200 && w.Code != 204) {
			t.Fatal(method, w.Code)
		}
	}
}

func TestVersionUpdateRouteGate(t *testing.T) {
	auth, _ := wiredAuth(t)
	r := NewWithAuth(nil, auth)
	// ALWAYS_PROTECTED: sem autenticação é 401 para POST e OPTIONS.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/version/update", nil))
	if w.Code != 401 {
		t.Fatalf("anon post %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/api/version/update", nil))
	if w.Code != 401 {
		t.Fatalf("anon options %d", w.Code)
	}
	token, _ := auth.Sessions.Create(nil)
	cookie := &http.Cookie{Name: "auth_token", Value: token}
	req := httptest.NewRequest("POST", "/api/version/update", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "Update is only available in production build (vansrouter CLI)") {
		t.Fatalf("update gate %d %s", w.Code, w.Body.String())
	}
	req = httptest.NewRequest("OPTIONS", "/api/version/update", nil)
	req.AddCookie(cookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 && w.Code != 204 {
		t.Fatalf("authed options %d", w.Code)
	}
}

func TestHeadroomRoutesLocalOnly(t *testing.T) {
	auth, _ := wiredAuth(t)
	r := NewWithNodes(nil, auth, nil, &handlers.Providers{Headroom: &handlers.HeadroomRoutes{H: &handlers.Headroom{DataDir: t.TempDir()}}})
	methods := map[string]string{
		"/api/headroom/status":  "GET",
		"/api/headroom/start":   "POST",
		"/api/headroom/stop":    "POST",
		"/api/headroom/restart": "POST",
		"/api/headroom/extras":  "GET",
	}
	for path, method := range methods {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != 401 && w.Code != 403 && w.Code != 500 {
			t.Fatalf("%s %s anon espera guarda local: %d %s", method, path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/headroom/nonexistent", nil))
	if w.Code != 404 {
		t.Fatalf("rota não registrada %d", w.Code)
	}
}

func TestVersionShutdownRouteRegistered(t *testing.T) {
	auth, _ := wiredAuth(t)
	r := NewWithAuth(nil, auth)
	// ALWAYS_PROTECTED exige 401 antes do gate/exit.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/version/shutdown", nil))
	if w.Code != 401 {
		t.Fatalf("anon %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/api/version/shutdown", nil))
	if w.Code == 404 {
		t.Fatal("options não registrada")
	}
}

func TestV1BetaModelsRoutePublic(t *testing.T) {
	auth, _ := wiredAuth(t)
	r := NewWithAuth(nil, auth)
	// Public LLM prefix: loopback do peer TCP é confiável; remoto exige key.
	loopback := httptest.NewRequest("GET", "/api/v1beta/models", nil)
	loopback.RemoteAddr = "127.0.0.1:5000"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, loopback)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"models"`) {
		t.Fatalf("v1beta %d", w.Code)
	}
	remote := httptest.NewRequest("GET", "/api/v1beta/models", nil)
	remote.RemoteAddr = "203.0.113.5:5000"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, remote)
	if w.Code != 401 {
		t.Fatalf("remoto %d %s", w.Code, w.Body.String())
	}
	loopbackNoAPI := httptest.NewRequest("GET", "/v1beta/models", nil)
	loopbackNoAPI.RemoteAddr = "127.0.0.1:5000"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, loopbackNoAPI)
	if w.Code != 200 {
		t.Fatalf("prefixo sem api %d", w.Code)
	}
}

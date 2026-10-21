package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

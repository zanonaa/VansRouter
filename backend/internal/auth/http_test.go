package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testHTTP(t *testing.T) *HTTP {
	t.Helper()
	s, err := LoadSessions(t.TempDir(), "segredo-ficticio-para-testes")
	if err != nil {
		t.Fatal(err)
	}
	return &HTTP{Sessions: s, Settings: func(context.Context) (map[string]any, error) { return map[string]any{}, nil }, Limiter: NewLimiter(), Initial: "123456", InitialValid: true, CheckConfigured: true}
}
func request(method, path, body, remote string) *http.Request {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = remote
	return r
}
func TestLoginAndCheck(t *testing.T) {
	h := testHTTP(t)
	w := httptest.NewRecorder()
	h.Login(w, request("POST", "/api/auth/login", `{"password":"123456"}`, "127.0.0.1:1234"))
	if w.Code != 200 || len(w.Result().Cookies()) != 1 || w.Result().Cookies()[0].Name != "auth_token" {
		t.Fatalf("login local: %d %s", w.Code, w.Body.String())
	}
	cookie := w.Result().Cookies()[0]
	r := request("GET", "/api/auth/check", "", "127.0.0.1:1234")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.Check(w, r)
	if w.Code != 200 {
		t.Fatal("sessão não aceita")
	}
	h.CheckConfigured = false
	w = httptest.NewRecorder()
	h.Check(w, r)
	if w.Code != 401 || !strings.Contains(w.Body.String(), `"authenticated":false`) {
		t.Fatal("diferença JWT_SECRET não preservada")
	}
	w = httptest.NewRecorder()
	h.Check(w, request("GET", "/api/auth/check", "", "127.0.0.1:1234"))
	if w.Code != 401 || !strings.Contains(w.Body.String(), "Unauthorized") {
		t.Fatal("barreira ausente")
	}
}
func TestRemoteDefaultOnlyGetsGrant(t *testing.T) {
	for _, remote := range []string{"192.0.2.10:1234", "127.0.0.1:1234"} {
		h := testHTTP(t)
		r := request("POST", "/api/auth/login", `{"password":"123456"}`, remote)
		if strings.HasPrefix(remote, "127.") {
			r.Header.Set("X-Forwarded-For", "192.0.2.10")
		}
		w := httptest.NewRecorder()
		h.Login(w, r)
		cookies := w.Result().Cookies()
		if w.Code != 200 || len(cookies) != 1 || cookies[0].Name != "password_change" || !h.Sessions.VerifyPasswordChange(cookies[0].Value) {
			t.Fatal("sessão completa concedida ao cliente remoto/proxy")
		}
	}
}
func TestLoginFailures(t *testing.T) {
	h := testHTTP(t)
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		h.Login(w, request("POST", "/api/auth/login", `{"password":"errada"}`, "192.0.2.10:1234"))
		want := 401
		if i == 4 {
			want = 429
		}
		if w.Code != want {
			t.Fatalf("falha %d: %d", i, w.Code)
		}
		if i == 4 && w.Header().Get("Retry-After") == "" {
			t.Fatal("Retry-After ausente")
		}
	}
	w := httptest.NewRecorder()
	h.Login(w, request("POST", "/api/auth/login", `{"password":"123456"}`, "192.0.2.10:1234"))
	if w.Code != 429 {
		t.Fatal("bloqueio ignorado")
	}
	for _, body := range []string{"null", "{", ""} {
		w := httptest.NewRecorder()
		h.Login(w, request("POST", "/api/auth/login", body, "192.0.2.11:1234"))
		if w.Code != 500 {
			t.Fatal("JSON inválido divergente")
		}
	}
}
func TestLoginPolicyAndOIDC(t *testing.T) {
	h := testHTTP(t)
	h.InitialValid = false
	w := httptest.NewRecorder()
	h.Login(w, request("POST", "/api/auth/login", `{}`, "127.0.0.1:1234"))
	if w.Code != 503 {
		t.Fatal("política inicial ignorada")
	}
	h.Settings = func(context.Context) (map[string]any, error) {
		return map[string]any{"authMode": "oidc", "oidcIssuerUrl": "https://idp.example", "oidcClientId": "id", "oidcClientSecret": "ficticio"}, nil
	}
	w = httptest.NewRecorder()
	h.Login(w, request("POST", "/api/auth/login", `{}`, "127.0.0.1:1234"))
	if w.Code != 403 {
		t.Fatal("OIDC exclusivo ignorado")
	}
}
func TestStatusAndLogout(t *testing.T) {
	h := testHTTP(t)
	w := httptest.NewRecorder()
	h.Status(w, request("GET", "/api/auth/status", "", "127.0.0.1:1234"))
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if len(body) != 10 || body["requireLogin"] != true || body["oidcName"] != nil || body["displayName"] != "Password user" {
		t.Fatalf("status: %v", body)
	}
	h.Settings = func(context.Context) (map[string]any, error) { return nil, errors.New("falha privada") }
	w = httptest.NewRecorder()
	h.Status(w, request("GET", "/api/auth/status", "", "127.0.0.1:1234"))
	if w.Code != 200 || strings.Contains(w.Body.String(), "privada") {
		t.Fatal("fallback divergente")
	}
	w = httptest.NewRecorder()
	h.Logout(w, request("POST", "/api/auth/logout", "", "127.0.0.1:1234"))
	if w.Code != 200 || len(w.Result().Cookies()) != 4 {
		t.Fatal("logout divergente")
	}
	for _, cookie := range w.Result().Cookies() {
		if cookie.MaxAge != -1 {
			t.Fatal("cookie não removido")
		}
	}
}

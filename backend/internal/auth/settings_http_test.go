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

var errFalhaPrivada = errors.New("falha privada acidental")

type writerStub struct {
	calls   int
	last    string
	failure error
}

func (w *writerStub) write(_ context.Context, hash string) (map[string]any, error) {
	if w.failure != nil {
		return nil, w.failure
	}
	w.calls++
	w.last = hash
	return map[string]any{}, nil
}

func settingsHTTP(t *testing.T, settings map[string]any) (*HTTP, *writerStub) {
	t.Helper()
	h := testHTTP(t)
	stub := &writerStub{}
	h.ChangeP = stub.write
	h.WriteSettings = func(_ context.Context, updates map[string]any) (map[string]any, error) {
		if stub.failure != nil {
			return nil, stub.failure
		}
		stub.calls++
		for k, v := range updates {
			if v == nil {
				delete(settings, k)
				continue
			}
			settings[k] = v
		}
		return settings, nil
	}
	h.Settings = func(context.Context) (map[string]any, error) { return settings, nil }
	return h, stub
}

func grantCookie(t *testing.T, h *HTTP) *http.Cookie {
	t.Helper()
	token, err := h.Sessions.CreatePasswordChange()
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "password_change", Value: token}
}

func sessionCookie(t *testing.T, h *HTTP) *http.Cookie {
	t.Helper()
	token, err := h.Sessions.Create(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "auth_token", Value: token}
}

func TestSettingsPatchGrantFlow(t *testing.T) {
	store := map[string]any{}
	h, stub := settingsHTTP(t, store)
	cookie := grantCookie(t, h)
	r := request("PATCH", "/api/settings", `{"newPassword":"senha-longa-para-grant","currentPassword":"123456"}`, "192.0.2.10:1234")
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.SettingsPatch(w, r)
	if w.Code != 200 || stub.calls != 1 {
		t.Fatalf("grant troca: %d %s", w.Code, w.Body.String())
	}
	hash, _ := store["password"].(string)
	if !VerifyPassword("senha-longa-para-grant", hash, "") {
		t.Fatal("hash divergente")
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["hasPassword"] != true || body["oidcConfigured"] != false || body["password"] != nil {
		t.Fatalf("resposta segura divergente: %v", body)
	}
	// currentPassword divergente da senha inicial → 401 (recusa estrita do original).
	cookie2 := grantCookie(t, h)
	w = httptest.NewRecorder()
	r = request("PATCH", "/api/settings", `{"newPassword":"senha-longa-para-grant","currentPassword":"errada"}`, "192.0.2.10:1234")
	r.AddCookie(cookie2)
	h.SettingsPatch(w, r)
	if w.Code != 401 || !strings.Contains(w.Body.String(), invalidCurrentMessage) {
		t.Fatalf("current divergente: %d %s", w.Code, w.Body.String())
	}
	// Tipos não string em currentPassword também são recusados como divergentes.
	w = httptest.NewRecorder()
	r = request("PATCH", "/api/settings", `{"newPassword":"senha-longa-para-grant","currentPassword":123456}`, "192.0.2.10:1234")
	r.AddCookie(cookie2)
	h.SettingsPatch(w, r)
	if w.Code != 401 {
		t.Fatalf("current numérico: %d %s", w.Code, w.Body.String())
	}
}

func TestSettingsPatchGrantRejections(t *testing.T) {
	h, stub := settingsHTTP(t, map[string]any{})
	cookie := grantCookie(t, h)
	for _, tt := range []struct {
		body, needle string
		status       int
	}{
		{`{"newPassword":"senha-longa-para-grant"}`, mustChangeOnly, 403},
		{`{"newPassword":"senha-longa-para-grant","currentPassword":"123456","theme":"dark"}`, mustChangeOnly, 403},
		{`{"currentPassword":"123456"}`, mustChangeOnly, 403},
		{`{"password":"x"}`, mustChangeOnly, 403},
		{`{"newPassword":"curta","currentPassword":"123456"}`, weakPasswordMessage, 400},
		{`{"newPassword":"1234567890123","currentPassword":"errada"}`, invalidCurrentMessage, 401},
	} {
		r := request("PATCH", "/api/settings", tt.body, "192.0.2.10:1234")
		r.AddCookie(cookie)
		w := httptest.NewRecorder()
		h.SettingsPatch(w, r)
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("%s: %d %s", tt.body, w.Code, w.Body.String())
		}
	}
	if stub.calls != 0 {
		t.Fatal("recusas não devem gravar")
	}
}

func TestSettingsPatchSessionFlow(t *testing.T) {
	store := map[string]any{}
	h, stub := settingsHTTP(t, store)
	cookie := sessionCookie(t, h)
	// Primeira definição: currentPassword legado "123456" é aceito (paridade original).
	r := request("PATCH", "/api/settings", `{"newPassword":"senha-longa-primeira","currentPassword":"123456"}`, "127.0.0.1:1234")
	r.AddCookie(cookie)
	w := httptest.NewRecorder()
	h.SettingsPatch(w, r)
	if w.Code != 200 || stub.calls != 1 {
		t.Fatalf("primeira definição: %d %s", w.Code, w.Body.String())
	}
	hash := stringOf(store["password"])
	// Hash existente: currentPassword ausente → 400.
	r = request("PATCH", "/api/settings", `{"newPassword":"senha-longa-segunda"}`, "127.0.0.1:1234")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.SettingsPatch(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Current password required") {
		t.Fatalf("current ausente: %d %s", w.Code, w.Body.String())
	}
	// Hash existente: current divergente → 401.
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"password": hash}, nil }
	r = request("PATCH", "/api/settings", `{"newPassword":"senha-longa-segunda","currentPassword":"123456"}`, "127.0.0.1:1234")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.SettingsPatch(w, r)
	if w.Code != 401 || !strings.Contains(w.Body.String(), invalidCurrentMessage) {
		t.Fatalf("current divergente: %d %s", w.Code, w.Body.String())
	}
	// Correta → 200 e resposta sem segredos.
	r = request("PATCH", "/api/settings", `{"newPassword":"senha-longa-segunda","currentPassword":"senha-longa-primeira"}`, "127.0.0.1:1234")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.SettingsPatch(w, r)
	if w.Code != 200 || stub.calls != 2 || strings.Contains(w.Body.String(), `"password"`) {
		t.Fatalf("troca com hash: %d %s", w.Code, w.Body.String())
	}
	// Chave comum é gravada (não mais 501 neste lote).
	r = request("PATCH", "/api/settings", `{"theme":"dark","requireLogin":false}`, "127.0.0.1:1234")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.SettingsPatch(w, r)
	if w.Code != 200 || store["theme"] != "dark" || store["requireLogin"] != false {
		t.Fatalf("escrita geral: %d %s %v", w.Code, w.Body.String(), store)
	}
	if strings.Contains(w.Body.String(), "password") {
		t.Fatal("segredo na resposta")
	}
	// Chave com efeito colateral não migrado continua 501 sem gravar.
	before := stub.calls
	r = request("PATCH", "/api/settings", `{"outboundProxyUrl":"http://proxy"}`, "127.0.0.1:1234")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.SettingsPatch(w, r)
	if w.Code != 501 || stub.calls != before || store["outboundProxyUrl"] != nil {
		t.Fatalf("efeito colateral: %d %d %v", w.Code, stub.calls, store["outboundProxyUrl"])
	}
	// Corpo apenas com chave protegida: gravação vazia como no original.
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{}, nil }
	r = request("PATCH", "/api/settings", `{"password":"tentativa"}`, "127.0.0.1:1234")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.SettingsPatch(w, r)
	if w.Code != 200 || stub.calls != before+1 || strings.Contains(w.Body.String(), `"password":"tentativa"`) {
		t.Fatalf("mass assignment: %d %s", w.Code, w.Body.String())
	}
	// oidcClientSecret vazio é removido, não gravado.
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{}, nil }
	r = request("PATCH", "/api/settings", `{"oidcClientSecret":""}`, "127.0.0.1:1234")
	r.AddCookie(cookie)
	w = httptest.NewRecorder()
	h.SettingsPatch(w, r)
	if w.Code != 200 {
		t.Fatalf("oidcClientSecret vazio: %d %s", w.Code, w.Body.String())
	}
	// Corpo inválido.
	for _, payload := range []string{"null", "{", ""} {
		r = request("PATCH", "/api/settings", payload, "127.0.0.1:1234")
		r.AddCookie(cookie)
		w = httptest.NewRecorder()
		h.SettingsPatch(w, r)
		if w.Code != 500 {
			t.Fatalf("JSON inválido: %d", w.Code)
		}
	}
}

func TestSettingsGet(t *testing.T) {
	h, _ := settingsHTTP(t, map[string]any{"password": "hash", "oidcIssuerUrl": "https://idp.example", "oidcClientId": "id", "oidcClientSecret": "ficticio", "requireLogin": true, "opcaoDesconhecida": float64(7)})
	h.EnableRequestLogs = true
	r := request("GET", "/api/settings", "", "127.0.0.1:1234")
	w := httptest.NewRecorder()
	h.SettingsGet(w, r)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["hasPassword"] != true || body["oidcConfigured"] != true || body["opcaoDesconhecida"] != float64(7) || body["enableRequestLogs"] != true {
		t.Fatalf("get seguro divergente: %v", body)
	}
	if _, leak := body["oidcClientSecret"]; leak {
		t.Fatal("segredo exposto")
	}
	if strings.Contains(w.Body.String(), "ficticio") {
		t.Fatal("segredo vazou no corpo")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("cabeçalho divergente")
	}
	h.Settings = func(context.Context) (map[string]any, error) { return nil, errFalhaPrivada }
	w = httptest.NewRecorder()
	h.SettingsGet(w, r)
	if w.Code != 500 || strings.Contains(w.Body.String(), "privada") {
		t.Fatalf("erro saneado divergente: %d %s", w.Code, w.Body.String())
	}
}

func TestSettingsGuards(t *testing.T) {
	h, _ := settingsHTTP(t, map[string]any{"requireLogin": true})
	next := func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]bool{"ok": true}) }
	// Sem cookie → 401 com mensagem idêntica ao guard original.
	w := httptest.NewRecorder()
	h.SettingsGuard(next)(w, request("PATCH", "/api/settings", `{}`, "127.0.0.1:1234"))
	if w.Code != 401 || !strings.Contains(w.Body.String(), "Unauthorized") {
		t.Fatalf("guard divergente: %d %s", w.Code, w.Body.String())
	}
	// requireLogin=false libera leitura.
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": false}, nil }
	w = httptest.NewRecorder()
	h.SettingsGuard(next)(w, request("GET", "/api/settings", "", "127.0.0.1:1234"))
	if w.Code != 200 {
		t.Fatalf("requireLogin=false: %d", w.Code)
	}
	// Grant só liberará PATCH.
	h, _ = settingsHTTP(t, map[string]any{"requireLogin": true})
	grant := grantCookie(t, h)
	r := request("GET", "/api/settings", "", "192.0.2.10:1234")
	r.AddCookie(grant)
	w = httptest.NewRecorder()
	h.SettingsGuard(next)(w, r)
	if w.Code != 401 {
		t.Fatal("grant liberou GET")
	}
	r = request("PATCH", "/api/settings", `{}`, "192.0.2.10:1234")
	r.AddCookie(grant)
	w = httptest.NewRecorder()
	h.SettingsGuard(next)(w, r)
	if w.Code != 200 {
		t.Fatalf("grant PATCH recusado: %d", w.Code)
	}
}

func TestLocalOnlyGuardAndReset(t *testing.T) {
	h, stub := settingsHTTP(t, map[string]any{})
	next := func(w http.ResponseWriter, r *http.Request) { reply(w, 200, map[string]bool{"ok": true}) }
	// Remoto → 403 com mensagem idêntica ao dashboardGuard.
	r := request("POST", "/api/auth/reset-password", "{\"newPassword\":\"senha-longa-teste\"}", "192.0.2.10:1234")
	w := httptest.NewRecorder()
	h.LocalOnlyGuard(next)(w, r)
	if w.Code != 403 || !strings.Contains(w.Body.String(), "Local only: CLI token required") {
		t.Fatalf("local-only divergente: %d %s", w.Code, w.Body.String())
	}
	// Local sem sessão → 403.
	w = httptest.NewRecorder()
	h.LocalOnlyGuard(next)(w, request("POST", "/api/auth/reset-password", "", "127.0.0.1:1234"))
	if w.Code != 403 {
		t.Fatal("local sem sessão passou")
	}
	// requireLogin=false + local → passa; remoto continua bloqueado.
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": false}, nil }
	w = httptest.NewRecorder()
	h.LocalOnlyGuard(next)(w, request("POST", "/api/auth/reset-password", "", "127.0.0.1:1234"))
	if w.Code != 200 {
		t.Fatalf("requireLogin=false local: %d", w.Code)
	}
	w = httptest.NewRecorder()
	h.LocalOnlyGuard(next)(w, request("POST", "/api/auth/reset-password", "", "192.0.2.10:1234"))
	if w.Code != 403 {
		t.Fatal("remoto passou com requireLogin=false")
	}
	// Sessão válida local → ResetPassword.
	token, err := h.Sessions.Create(nil)
	if err != nil {
		t.Fatal(err)
	}
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{}, nil }
	r = request("POST", "/api/auth/reset-password", `{"newPassword":"senha-longa-reset"}`, "127.0.0.1:1234")
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w = httptest.NewRecorder()
	h.ResetPassword(w, r)
	if w.Code != 200 || stub.calls == 0 || !VerifyPassword("senha-longa-reset", stub.last, "") {
		t.Fatalf("reset: %d %s", w.Code, w.Body.String())
	}
	store2 := map[string]any{}
	h2, _ := settingsHTTP(t, store2)
	w = httptest.NewRecorder()
	r = request("PATCH", "/api/settings", `{"newPassword":"senha-longa-reset","currentPassword":"123456"}`, "127.0.0.1:1234")
	r.AddCookie(sessionCookie(t, h2))
	h2.SettingsPatch(w, r)
	if w.Code != 200 || !VerifyPassword("senha-longa-reset", stringOf(store2["password"]), "") {
		t.Fatalf("reset via patch: %d %s", w.Code, w.Body.String())
	}
	// Corpo fraco/inválido.
	for _, tt := range []struct {
		body, needle string
		status       int
	}{
		{`{"newPassword":"curta"}`, weakPasswordMessage, 400},
		{`{}`, weakPasswordMessage, 400},
		{"{", "Invalid JSON body", 400},
	} {
		r = request("POST", "/api/auth/reset-password", tt.body, "127.0.0.1:1234")
		r.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
		w = httptest.NewRecorder()
		h.ResetPassword(w, r)
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("%s: %d %s", tt.body, w.Code, w.Body.String())
		}
	}
	// Falha de gravação é saneada.
	stub.failure = errFalhaPrivada
	r = request("POST", "/api/auth/reset-password", `{"newPassword":"senha-longa-reset"}`, "127.0.0.1:1234")
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: token})
	w = httptest.NewRecorder()
	h.ResetPassword(w, r)
	if w.Code != 500 || strings.Contains(w.Body.String(), "privada") {
		t.Fatalf("falha de gravação: %d %s", w.Code, w.Body.String())
	}
}

package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAlwaysProtectedDoesNotAcceptDisabledLogin(t *testing.T) {
	h := testHTTP(t)
	h.Machine = NewMachineID(t.TempDir())
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": false}, nil }
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	for _, path := range []string{"/api/shutdown", "/api/settings/database", "/api/version/update"} {
		r := request("POST", path, "", "192.0.2.10:1234")
		w := httptest.NewRecorder()
		h.Middleware(next).ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("%s bypass: %d", path, w.Code)
		}
		r.Header.Set("X-9r-cli-token", h.CLIToken())
		w = httptest.NewRecorder()
		h.Middleware(next).ServeHTTP(w, r)
		if w.Code != 204 {
			t.Fatalf("CLI %s: %d", path, w.Code)
		}
	}
}
func TestInternalGuardsAcceptCLI(t *testing.T) {
	h := testHTTP(t)
	h.Machine = NewMachineID(t.TempDir())
	next := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }
	for _, guard := range []func(http.HandlerFunc) http.HandlerFunc{h.SettingsGuard, h.LocalOnlyGuard} {
		r := request("POST", "/api/auth/reset-password", "", "192.0.2.10:1234")
		r.Header.Set("X-9r-cli-token", h.CLIToken())
		w := httptest.NewRecorder()
		guard(next)(w, r)
		if w.Code != 204 {
			t.Fatalf("valid CLI blocked %d", w.Code)
		}
		r.Header.Set("X-9r-cli-token", "invalid")
		w = httptest.NewRecorder()
		guard(next)(w, r)
		if w.Code == 204 {
			t.Fatal("invalid CLI allowed")
		}
	}
}
func TestSettingsPatchDisabledLogin(t *testing.T) {
	h := testHTTP(t)
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": false}, nil }
	written := false
	h.WriteSettings = func(_ context.Context, updates map[string]any) (map[string]any, error) {
		written = true
		return updates, nil
	}
	w := httptest.NewRecorder()
	h.SettingsGuard(h.SettingsPatch)(w, request("PATCH", "/api/settings", `{"theme":"dark"}`, "192.0.2.10:1234"))
	if w.Code != 200 || !written || !strings.Contains(w.Body.String(), "dark") {
		t.Fatalf("disabled login %d %s", w.Code, w.Body.String())
	}
}

package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestZedImportAuthAndOrganization(t *testing.T) {
	fail := false
	calls := 0
	var saved map[string]any
	expectedSystem := "provided-system"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.Header.Get("Authorization") != "42 dummy-token" {
			t.Error("Zed auth scheme")
		}
		if expectedSystem != "" && r.Header.Get("x-zed-system-id") != expectedSystem {
			t.Error("system ID")
		}
		if expectedSystem == "" && len(r.Header.Get("x-zed-system-id")) != 36 {
			t.Error("missing generated UUID")
		}
		w.Header().Set("Content-Type", "application/json")
		if fail {
			w.WriteHeader(401)
			_, _ = w.Write([]byte(`{"error":{"message":"invalid session"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"email":"op@example.test","display_name":"Operator","organizations":[{"id":"team"},{"id":"personal","is_personal":true}]}`))
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig { return &providers.OAuthConfig{UserInfoURL: server.URL} }, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	for _, payload := range []string{`{}`, `{"accessToken":"token"}`} {
		w := httptest.NewRecorder()
		h.ZedImport(w, httptest.NewRequest("POST", "/", strings.NewReader(payload)))
		if w.Code != 400 || calls != 0 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	payload := `{"accessToken":" dummy-token ","userId":42,"systemId":" provided-system "}`
	w := httptest.NewRecorder()
	h.ZedImport(w, httptest.NewRequest("POST", "/", strings.NewReader(payload)))
	if w.Code != 200 || saved["email"] != "op@example.test" || saved["displayName"] != "Operator" || strings.Contains(w.Body.String(), "dummy-token") {
		t.Fatal(w.Code, w.Body.String(), saved)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["organizationId"] != "personal" || specific["userId"] != "42" || specific["systemId"] != "provided-system" {
		t.Fatal(specific)
	}
	fail = true
	saved = nil
	w = httptest.NewRecorder()
	h.ZedImport(w, httptest.NewRequest("POST", "/", strings.NewReader(payload)))
	if w.Code != 401 || saved != nil || !strings.Contains(w.Body.String(), "invalid session") {
		t.Fatal(w.Code, w.Body.String())
	}
	fail = false
	expectedSystem = ""
	w = httptest.NewRecorder()
	h.ZedImport(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"accessToken":"dummy-token","userId":"42"}`)))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestZedOrganizationShapes(t *testing.T) {
	for _, tt := range []struct {
		user map[string]any
		want string
	}{{map[string]any{"default_organization_id": []any{"default"}}, "default"}, {map[string]any{"defaultOrganizationId": map[string]any{"id": "object"}}, "object"}, {map[string]any{"organizations": []any{map[string]any{"id": "first"}}}, "first"}, {map[string]any{}, ""}} {
		if got := zedOrganization(tt.user); got != tt.want {
			t.Fatalf("%v: %s want %s", tt.user, got, tt.want)
		}
	}
}

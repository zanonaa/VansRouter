package router

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestOAuthEngineContract(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	connections := database.Connections{DB: db}
	authentication, _ := wiredAuth(t)
	r := NewWithNodes(nil, authentication, nil, &handlers.Providers{OAuth: &handlers.OAuthEngine{CreateConnection: connections.Create}})
	for _, tt := range []struct {
		method, path, body string
		status             int
		needle             string
	}{
		{"GET", "/api/oauth/claude/authorize?redirect_uri=http://localhost:8080/cb", "", 200, `"codeVerifier"`},
		{"GET", "/api/oauth/claude/unknown", "", 400, "Unknown action"},
		{"GET", "/api/oauth/claude/start-proxy?app_port=1234", "", 400, "Proxy only supported"},
		{"GET", "/api/oauth/claude/poll-status", "", 400, "Proxy only supported"},
		{"GET", "/api/oauth/claude/stop-proxy", "", 400, "Proxy only supported"},
		{"GET", "/api/oauth/claude/device-code", "", 501, "Device code flow not yet migrated"},
		{"POST", "/api/oauth/claude/exchange", `invalid`, 400, "Invalid or empty request body"},
		{"POST", "/api/oauth/claude/exchange", `{}`, 400, "Missing required fields"},
		{"POST", "/api/oauth/claude/exchange", `{"code":"abc","redirectUri":"http://localhost:8080/cb"}`, 400, "Missing required fields"},
		{"POST", "/api/oauth/cline/exchange", `{"code":"abc","redirectUri":"http://localhost:8080/cb"}`, 501, "not yet migrated"},
		{"POST", "/api/oauth/claude/exchange", `{"code":"abc","redirectUri":"http://localhost:8080/cb","codeVerifier":"v"}`, 501, "not yet migrated"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("%s %s: %d %s", tt.method, tt.path, w.Code, w.Body.String())
		}
	}
	// JWT cru de acesso (Codex) salva como access_token com claims.
	jwt := buildFakeJWT(map[string]any{"email": "op@example.test", "chatgpt_account_id": "acct-1", "plan_type": "plus"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/codex/exchange", strings.NewReader(`{"code":"`+jwt+`"}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"success":true`) || !strings.Contains(w.Body.String(), "op@example.test") {
		t.Fatalf("jwt: %d %s", w.Code, w.Body.String())
	}
	connection, err := connections.List(ctx, "codex", nil)
	if err != nil || len(connection) != 1 {
		t.Fatalf("saved: %v %v", connection, err)
	}
	specific := connection[0]["providerSpecificData"].(map[string]any)
	if specific["authMethod"] != "access_token" || specific["chatgptAccountId"] != "acct-1" {
		t.Fatalf("claims: %v", specific)
	}
	_ = json.Marshal
}

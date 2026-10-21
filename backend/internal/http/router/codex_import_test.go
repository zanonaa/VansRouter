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

func TestCodexImportToken(t *testing.T) {
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
	for _, payload := range []string{`{}`, `{"accessToken":123}`, `{"accessToken":""}`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/codex/import-token", strings.NewReader(payload)))
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	token := buildFakeJWT(map[string]any{"https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "workspace", "chatgpt_plan_type": "plus"}, "https://api.openai.com/profile": map[string]any{"email": "op@example.test"}, "exp": float64(1900000000)})
	payload, _ := json.Marshal(map[string]string{"accessToken": " " + token + " "})
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/codex/import-token", strings.NewReader(string(payload))))
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"workspace":"workspace"`) || strings.Contains(w.Body.String(), token) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	list, err := connections.List(ctx, "codex", nil)
	if err != nil || len(list) != 2 {
		t.Fatal("access_token imports must not dedup", len(list), err)
	}
	specific := list[0]["providerSpecificData"].(map[string]any)
	if list[0]["accessToken"] != token || list[0]["email"] != "op@example.test" || specific["jwtExp"] != float64(1900000000) {
		t.Fatal(list[0])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/codex/import-token", strings.NewReader(`{"accessToken":"opaque-dummy","name":"Manual"}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"workspace":null`) {
		t.Fatal(w.Code, w.Body.String())
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/codex/import-token", strings.NewReader(string(payload))))
	if w.Code != 401 {
		t.Fatal("unguarded import")
	}
}

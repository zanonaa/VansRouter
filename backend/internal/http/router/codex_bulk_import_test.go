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

func TestCodexBulkImport(t *testing.T) {
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
	token := buildFakeJWT(map[string]any{"email": "op@example.test", "https://api.openai.com/auth": map[string]any{"chatgpt_account_id": "workspace", "chatgpt_plan_type": "plus"}})
	payload, _ := json.Marshal(map[string]any{"accounts": []any{map[string]any{"id": "attacker-id", "provider": "other", "authType": "apikey", "createdAt": "bad", "accessToken": token, "expiresIn": 3600}, nil, map[string]any{"name": "missing"}, map[string]any{"accessToken": token}}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/codex/bulk-import", strings.NewReader(string(payload))))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"success":2`) || !strings.Contains(w.Body.String(), `"failed":2`) || strings.Contains(w.Body.String(), token) {
		t.Fatal(w.Code, w.Body.String())
	}
	list, err := connections.List(ctx, "codex", nil)
	if err != nil || len(list) != 1 {
		t.Fatal("workspace upsert failed", list, err)
	}
	item := list[0]
	if item["id"] == "attacker-id" || item["authType"] != "oauth" || item["email"] != "op@example.test" || item["lastRefreshAt"] == nil || item["expiresAt"] == nil {
		t.Fatal(item)
	}
	for _, raw := range []string{"null", "[]", "invalid"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/codex/bulk-import", strings.NewReader(raw)))
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/codex/bulk-import", strings.NewReader(`{"accessToken":"opaque","email":"unique@example.test"}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"success":1`) {
		t.Fatal(w.Code, w.Body.String())
	}
}

package router

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestGrokBulkImport(t *testing.T) {
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
	jwt := buildFakeJWT(map[string]any{"email": "op@example.test"})
	payload, _ := json.Marshal(map[string]any{"accounts": []any{
		map[string]any{"id": "do-not-use", "provider": "other", "authType": "apikey", "access_token": "dummy-access", "refresh_token": "dummy-refresh", "id_token": jwt, "expires_in": 3600, "name": "Imported", "providerSpecificData": map[string]any{"custom": true, "authMethod": "explicit"}},
		nil, map[string]any{"accessToken": 123}, map[string]any{"accessToken": "replacement", "email": "op@example.test", "expiresAt": "2027-01-01T00:00:00Z", "displayName": "Updated", "providerSpecificData": map[string]any{"custom": true}},
	}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/grok-cli/bulk-import", strings.NewReader(string(payload))))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"total":4`) || !strings.Contains(w.Body.String(), `"success":2`) || !strings.Contains(w.Body.String(), `"failed":2`) || strings.Contains(w.Body.String(), "dummy-access") || strings.Contains(w.Body.String(), jwt) {
		t.Fatal(w.Code, w.Body.String())
	}
	list, err := connections.List(ctx, "grok-cli", nil)
	if err != nil || len(list) != 1 {
		t.Fatal("email dedup", list, err)
	}
	item := list[0]
	if item["accessToken"] != "replacement" || item["id"] == "do-not-use" || item["authType"] != "oauth" || item["displayName"] != "Updated" || item["expiresAt"] != "2027-01-01T00:00:00Z" {
		t.Fatal(item)
	}
	// Um registro independente mantém a precedência do PSD explícito e o refresh.
	payload, _ = json.Marshal(map[string]any{"accessToken": "standalone", "email": "other@example.test", "refreshToken": "refresh", "expiresIn": 7200, "providerSpecificData": map[string]any{"authMethod": "overridden", "email": "specific@example.test"}})
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/grok-cli/bulk-import", strings.NewReader(string(payload))))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	list, err = connections.List(ctx, "grok-cli", nil)
	if err != nil || len(list) != 2 {
		t.Fatal(list, err)
	}
	found := false
	for _, row := range list {
		if row["email"] == "other@example.test" {
			found = true
			data := row["providerSpecificData"].(map[string]any)
			if data["authMethod"] != "overridden" || data["email"] != "specific@example.test" || row["refreshToken"] != "refresh" {
				t.Fatal(row)
			}
			expiry, err := time.Parse(time.RFC3339Nano, row["expiresAt"].(string))
			if err != nil || time.Until(expiry) < 7190*time.Second {
				t.Fatal(row)
			}
		}
	}
	if !found {
		t.Fatal("single import absent")
	}
	for _, raw := range []string{"[]", "null", "invalid"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/grok-cli/bulk-import", strings.NewReader(raw)))
		if w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/grok-cli/bulk-import", strings.NewReader(string(payload))))
	if w.Code != 401 {
		t.Fatal("unguarded import")
	}
}

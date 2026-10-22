package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func gatewayModelsFixture(t *testing.T) *Gateway {
	t.Helper()
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE settings(id INTEGER PRIMARY KEY,data TEXT); INSERT INTO settings VALUES(1,'{"requireApiKey":true}'); CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT); INSERT INTO providerConnections VALUES('o','openai','apikey',NULL,NULL,1,1,'{}','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	return &Gateway{DB: db}
}
func TestGatewayModelsAuthAndACL(t *testing.T) {
	g := gatewayModelsFixture(t)
	w := httptest.NewRecorder()
	g.Models(w, httptest.NewRequest("GET", "/api/v1/models", nil))
	if w.Code != 401 {
		t.Fatalf("missing auth %d", w.Code)
	}
	key, err := database.CreateApiKey(context.Background(), g.DB, "test", "machine")
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.UpdateApiKey(context.Background(), g.DB, ttsString(key["id"]), map[string]any{"allowedProviders": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("GET", "/api/v1/models", nil)
	request.Header.Set("Authorization", "Bearer "+ttsString(key["key"]))
	w = httptest.NewRecorder()
	g.Models(w, request)
	if w.Code != 200 {
		t.Fatalf("valid auth %d: %s", w.Code, w.Body.String())
	}
	var response struct {
		Data []any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) != 0 {
		t.Fatal("empty provider ACL exposed models")
	}
	_, err = database.UpdateApiKey(context.Background(), g.DB, ttsString(key["id"]), map[string]any{"allowedProviders": []any{"openai"}})
	if err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	g.Models(w, request)
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Data) == 0 {
		t.Fatal("active permitted provider missing")
	}
}
func TestGatewayPermissionEmptyDenies(t *testing.T) {
	if !gatewayPermission(nil, "allowedProviders", "openai") {
		t.Fatal("unrestricted denied")
	}
	if gatewayPermission(map[string]any{"allowedProviders": []any{}}, "allowedProviders", "openai") {
		t.Fatal("empty ACL allowed")
	}
	if !gatewayPermission(map[string]any{"allowedProviders": []any{"openai"}}, "allowedProviders", "openai") {
		t.Fatal("explicit ACL denied")
	}
}

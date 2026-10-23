package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func gatewayModelsFixture(t *testing.T) *Gateway {
	t.Helper()
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE settings(id INTEGER PRIMARY KEY,data TEXT); INSERT INTO settings VALUES(1,'{"requireApiKey":true}'); CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT); INSERT INTO providerConnections VALUES('o','openai','apikey',NULL,NULL,1,1,'{}','now','now'); CREATE TABLE combos(id TEXT PRIMARY KEY,name TEXT,kind TEXT,models TEXT,context_length REAL,createdAt TEXT,updatedAt TEXT);`)
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

func TestGatewayModelsListsCombos(t *testing.T) {
	g := gatewayModelsFixture(t)
	if _, err := g.DB.Exec(`INSERT INTO combos VALUES('c1','zn-test',NULL,'["openai/gpt"]',NULL,'now','now');
		INSERT INTO combos VALUES('c2','web-test','webFetch','[]',NULL,'now','now');`); err != nil {
		t.Fatal(err)
	}
	key, err := database.CreateApiKey(context.Background(), g.DB, "test", "machine")
	if err != nil {
		t.Fatal(err)
	}
	list := func() map[string]map[string]any {
		request := httptest.NewRequest("GET", "/api/v1/models", nil)
		request.Header.Set("Authorization", "Bearer "+ttsString(key["key"]))
		w := httptest.NewRecorder()
		g.Models(w, request)
		if w.Code != 200 {
			t.Fatalf("models %d: %s", w.Code, w.Body.String())
		}
		var body struct {
			Data []map[string]any `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		out := map[string]map[string]any{}
		for _, entry := range body.Data {
			out[ttsString(entry["id"])] = entry
		}
		return out
	}

	// Sem ACL o combo LLM precisa aparecer; o combo de mídia fica em /models/web.
	all := list()
	combo, ok := all["combo/zn-test"]
	if !ok || combo["owned_by"] != "combo" {
		t.Fatalf("combo LLM ausente do /v1/models: %v", all)
	}
	if _, ok := all["combo/web-test"]; ok {
		t.Fatal("combo de mídia listado como LLM")
	}

	router := chi.NewRouter()
	router.Get("/api/v1/models/{kind}", g.ModelsByKind)
	request := httptest.NewRequest("GET", "/api/v1/models/web", nil)
	request.Header.Set("Authorization", "Bearer "+ttsString(key["key"]))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, request)
	if !strings.Contains(w.Body.String(), "combo/web-test") {
		t.Fatalf("combo web ausente: %s", w.Body.String())
	}

	// allowedCombos vazio nega todos, como no Node (isComboAllowed).
	if _, err := database.UpdateApiKey(context.Background(), g.DB, ttsString(key["id"]), map[string]any{"allowedCombos": []any{}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := list()["combo/zn-test"]; ok {
		t.Fatal("ACL vazia expôs combo")
	}
	if _, err := database.UpdateApiKey(context.Background(), g.DB, ttsString(key["id"]), map[string]any{"allowedCombos": []any{"zn-test"}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := list()["combo/zn-test"]; !ok {
		t.Fatal("combo permitido ausente")
	}
}

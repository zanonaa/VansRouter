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

func TestProviderCreateHTTP(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT NOT NULL,authType TEXT NOT NULL,name TEXT,email TEXT,priority INTEGER,isActive INTEGER,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);
 CREATE TABLE providerNodes(id TEXT PRIMARY KEY,type TEXT,name TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);
 CREATE TABLE proxyPools(id TEXT PRIMARY KEY,isActive INTEGER,testStatus TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	connections := database.Connections{DB: db}
	nodes := database.Nodes{DB: db}
	pools := database.ProxyPools{DB: db}
	authentication, _ := wiredAuth(t)
	api := &handlers.Providers{CreateConnection: connections.Create, GetNode: nodes.Get, GetPool: pools.Get}
	r := NewWithNodes(nil, authentication, &handlers.Nodes{Store: nodes, Connections: connections}, api)
	for _, tt := range []struct {
		body, message string
		status        int
	}{
		{`{}`, "Invalid provider", 400},
		{`{"provider":"openai"}`, "API Key is required", 400},
		{`{"provider":"openai","connectionProxyEnabled":true}`, "Connection proxy URL is required", 400},
		{`{"provider":"openai","apiKey":"ficticia","proxyPoolId":"missing"}`, "Proxy pool not found", 400},
		{`{"provider":"openai-compatible-chat-missing","apiKey":"ficticia","name":"N"}`, "OpenAI Compatible node not found", 404},
		{`{`, "Failed to create provider", 500},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/providers", strings.NewReader(tt.body)))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.message) {
			t.Fatalf("%s: %d %s", tt.body, w.Code, w.Body.String())
		}
	}
	var id string
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/providers", strings.NewReader(`{"provider":" OpenAI ","apiKey":"secret-ficticio","name":"Same"}`)))
		if w.Code != 201 || strings.Contains(w.Body.String(), "secret-ficticio") {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
		var body struct {
			Connection map[string]any `json:"connection"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		next, _ := body.Connection["id"].(string)
		if i == 0 {
			id = next
		} else if id != next {
			t.Fatal("same name did not upsert")
		}
	}
	stored, err := connections.Get(ctx, id)
	if err != nil || stored["apiKey"] != "secret-ficticio" {
		t.Fatalf("persistence: %v %v", stored, err)
	}
	if _, err = nodes.Create(ctx, map[string]any{"id": "anthropic-compatible-test", "type": "anthropic-compatible", "name": "Node", "prefix": "n", "baseUrl": "https://example.test/v1"}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/providers", strings.NewReader(`{"provider":"anthropic-compatible-test","name":"Compat","apiKey":"test","providerSpecificData":{"custom":true}}`)))
	if w.Code != 201 || !strings.Contains(w.Body.String(), "https://example.test/v1") || !strings.Contains(w.Body.String(), `"custom":true`) {
		t.Fatalf("compatible: %d %s", w.Code, w.Body.String())
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/providers", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatalf("guard: %d", w.Code)
	}
}

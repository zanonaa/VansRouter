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

func TestNodeMutationsPropagateConnections(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE providerNodes(id TEXT PRIMARY KEY,type TEXT,name TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);
 CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT NOT NULL,authType TEXT NOT NULL,name TEXT,email TEXT,priority INTEGER,isActive INTEGER DEFAULT 1,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);
 INSERT INTO providerConnections VALUES('linked','node-id','apikey','Key',NULL,1,1,'{"apiKey":"ficticia","providerSpecificData":{"custom":true,"apiType":"old"}}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z');
 INSERT INTO providerConnections VALUES('other','other-provider','apikey','Other',NULL,1,1,'{}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z');`)
	if err != nil {
		t.Fatal(err)
	}
	nodes := database.Nodes{DB: db}
	connections := database.Connections{DB: db}
	if _, err = nodes.Create(ctx, map[string]any{"id": "node-id", "type": "anthropic-compatible", "name": "Old", "prefix": "old", "baseUrl": "https://old.test/v1"}); err != nil {
		t.Fatal(err)
	}
	authentication, _ := wiredAuth(t)
	r := NewWithNodes(nil, authentication, &handlers.Nodes{Store: nodes, Mutations: nodes, Connections: connections})
	for _, tt := range []struct {
		path, body, needle string
		status             int
	}{
		{"missing", `{}`, "Provider node not found", 404},
		{"node-id", `{}`, "Name is required", 400},
		{"node-id", `{"name":"New"}`, "Prefix is required", 400},
		{"node-id", `{"name":"New","prefix":"n"}`, "Base URL is required", 400},
		{"node-id", `{"name":" New ","prefix":" n ","baseUrl":"https://new.test/messages/"}`, "https://new.test/v1", 200},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/provider-nodes/"+tt.path, strings.NewReader(tt.body)))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("PUT %s: %d %s", tt.path, w.Code, w.Body.String())
		}
	}
	linked, err := connections.Get(ctx, "linked")
	if err != nil {
		t.Fatal(err)
	}
	specific := linked["providerSpecificData"].(map[string]any)
	if specific["custom"] != true || specific["nodeName"] != "New" || specific["prefix"] != "n" || specific["baseUrl"] != "https://new.test/v1" {
		t.Fatalf("propagation: %v", specific)
	}
	if _, exists := specific["apiType"]; exists {
		t.Fatalf("apiType deve ser omitido para anthropic: %v", specific)
	}
	if linked["apiKey"] != "ficticia" {
		t.Fatal("campo extra perdido")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/provider-nodes/node-id", nil))
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 200 || body["success"] != true {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	linked, err = connections.Get(ctx, "linked")
	if err != nil || linked != nil {
		t.Fatalf("connection órfã: %v %v", linked, err)
	}
	other, err := connections.Get(ctx, "other")
	if err != nil || other == nil {
		t.Fatal("connection não relacionada removida")
	}
	node, err := nodes.Get(ctx, "node-id")
	if err != nil || node != nil {
		t.Fatal("nó não removido")
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/provider-nodes/node-id", nil))
	if w.Code != 404 {
		t.Fatalf("missing delete: %d", w.Code)
	}
}

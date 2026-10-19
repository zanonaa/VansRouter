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

func TestNodesHTTP(t *testing.T) {
	ctx := context.Background()
	db, err := database.Open(ctx, t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE providerNodes(id TEXT PRIMARY KEY,type TEXT,name TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	authentication, _ := wiredAuth(t)
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	r := NewWithNodes(nil, authentication, &handlers.Nodes{Store: database.Nodes{DB: db}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/provider-nodes", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatalf("guard: %d", w.Code)
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": false}, nil }
	for _, tt := range []struct {
		body, needle string
		status       int
	}{
		{`{}`, "Name is required", 400},
		{`{"name":"n"}`, "Prefix is required", 400},
		{`{"name":"n","prefix":"p"}`, "Invalid OpenAI compatible API type", 400},
		{`{"name":123,"prefix":"p"}`, "Failed to create provider node", 500},
		{`{"name":"n","prefix":"p","type":"bad"}`, "Invalid provider node type", 400},
		{`{`, "Failed to create provider node", 500},
	} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/provider-nodes", strings.NewReader(tt.body)))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("%s: %d %s", tt.body, w.Code, w.Body.String())
		}
	}
	for _, tt := range []struct{ kind, api, base, want, idPrefix string }{
		{"openai-compatible", "chat", "", "https://api.openai.com/v1", "openai-compatible-chat-"},
		{"openai-compatible", "responses", " https://example.test/v1/ ", "https://example.test/v1/", "openai-compatible-responses-"},
		{"anthropic-compatible", "", " https://example.test/messages/ ", "https://example.test/v1", "anthropic-compatible-"},
		{"custom-embedding", "", " https://example.test/v1/embeddings/ ", "https://example.test/v1", "custom-embedding-"},
	} {
		payload, _ := json.Marshal(map[string]any{"name": " Teste ", "prefix": " p ", "type": tt.kind, "apiType": tt.api, "baseUrl": tt.base})
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/provider-nodes", strings.NewReader(string(payload))))
		if w.Code != 201 {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
		var result struct {
			Node map[string]any `json:"node"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		id, _ := result.Node["id"].(string)
		if !strings.HasPrefix(id, tt.idPrefix) || result.Node["baseUrl"] != tt.want || result.Node["name"] != "Teste" || result.Node["prefix"] != "p" {
			t.Fatalf("node: %v", result.Node)
		}
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/provider-nodes", nil))
	var list struct {
		Nodes []map[string]any `json:"nodes"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if w.Code != 200 || len(list.Nodes) != 4 {
		t.Fatalf("list: %d %s", w.Code, w.Body.String())
	}
	for _, method := range []string{"HEAD", "OPTIONS"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/provider-nodes", nil))
		if w.Body.Len() != 0 || (w.Code != 200 && w.Code != 204) {
			t.Fatalf("%s: %d %s", method, w.Code, w.Body.String())
		}
	}
}

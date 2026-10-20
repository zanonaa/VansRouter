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
 CREATE TABLE proxyPools(id TEXT PRIMARY KEY,isActive INTEGER,testStatus TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL);
 CREATE TABLE proxyPoolFitness(poolId TEXT, marker TEXT);`)
	if err != nil {
		t.Fatal(err)
	}
	connections := database.Connections{DB: db}
	nodes := database.Nodes{DB: db}
	pools := database.ProxyPools{DB: db}
	authentication, _ := wiredAuth(t)
	api := &handlers.Providers{CreateBulk: connections.CreateBulk, Pools: &handlers.ProxyPools{Store: pools, Connections: connections}, GetConnection: connections.Get, UpdateConnection: connections.Update, DeleteConnection: connections.Delete, CreateConnection: connections.Create, GetNode: nodes.Get, GetPool: pools.Get}
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
	if _, err = pools.Create(ctx, map[string]any{"id": "pool", "name": "Pool", "proxyUrl": "http://proxy.example.test"}); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		body, needle string
		status       int
	}{
		{`{"proxyPoolIds":["pool"],"proxyRotationStrategy":"invalid"}`, "Invalid proxy rotation strategy", 400},
		{`{"proxyPoolId":"missing"}`, "Proxy pool not found", 400},
		{`{"connectionProxyEnabled":true}`, "Connection proxy URL is required", 400},
		{`{"apiKey":"replacement","proxyPoolIds":["pool","pool"],"proxyRotationStrategy":"SMART","providerSpecificData":{"extra":true}}`, `"smart"`, 200},
	} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/providers/"+id, strings.NewReader(tt.body)))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("PUT: %d %s", w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "replacement") {
			t.Fatal("credential leaked")
		}
	}
	stored, err = connections.Get(ctx, id)
	if err != nil || stored["apiKey"] != "replacement" {
		t.Fatalf("updated: %v %v", stored, err)
	}
	specific := stored["providerSpecificData"].(map[string]any)
	if specific["extra"] != true || len(specific["proxyPoolIds"].([]any)) != 1 {
		t.Fatalf("pool merge: %v", specific)
	}
	boundResponse := httptest.NewRecorder()
	r.ServeHTTP(boundResponse, httptest.NewRequest("DELETE", "/api/proxy-pools/pool", nil))
	if boundResponse.Code != 409 || !strings.Contains(boundResponse.Body.String(), `"boundConnectionCount":1`) {
		t.Fatalf("bound pool deletion: %d %s", boundResponse.Code, boundResponse.Body.String())
	}
	usageResponse := httptest.NewRecorder()
	r.ServeHTTP(usageResponse, httptest.NewRequest("GET", "/api/proxy-pools?includeUsage=true", nil))
	if usageResponse.Code != 200 || !strings.Contains(usageResponse.Body.String(), `"boundConnectionCount":1`) {
		t.Fatalf("pool usage: %d %s", usageResponse.Code, usageResponse.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/providers/"+id, nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "replacement") {
		t.Fatalf("GET: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/providers/"+id, strings.NewReader(`{"proxyPoolId":null}`)))
	if w.Code != 200 {
		t.Fatalf("clear pool: %d %s", w.Code, w.Body.String())
	}
	stored, err = connections.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	specific = stored["providerSpecificData"].(map[string]any)
	for _, key := range []string{"proxyPoolId", "proxyPoolIds", "proxyRotationStrategy"} {
		if _, exists := specific[key]; exists {
			t.Fatalf("pool not cleared: %v", specific)
		}
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/providers/"+id, nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Connection deleted successfully") {
		t.Fatalf("DELETE: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/providers/"+id, nil))
	if w.Code != 404 {
		t.Fatalf("missing GET: %d", w.Code)
	}
	for _, tt := range []struct {
		method, path, body, needle string
		status                     int
	}{
		{"POST", "/api/proxy-pools", `{}`, "Name is required", 400},
		{"POST", "/api/proxy-pools", `{"name":"P"}`, "Proxy URL is required", 400},
		{"POST", "/api/proxy-pools", `{"name":"P","proxyUrl":"http://example.test","type":"bad"}`, `"type":"http"`, 201},
		{"PUT", "/api/proxy-pools/pool", `{"name":" Updated ","isActive":false}`, "Updated", 200},
		{"GET", "/api/proxy-pools?isActive=false", "", `"isActive":false`, 200},
		{"DELETE", "/api/proxy-pools/pool", "", `"success":true`, 200},
		{"GET", "/api/proxy-pools/pool", "", "Proxy pool not found", 404},
	} {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		if response.Code != tt.status || !strings.Contains(response.Body.String(), tt.needle) {
			t.Fatalf("pool %s %s: %d %s", tt.method, tt.path, response.Code, response.Body.String())
		}
	}
	for _, tt := range []struct {
		body, needle string
		status       int
	}{
		{`{"items":[]}`, "items must contain 1-500 connections", 400},
		{`{"items":[{"provider":"openai","name":"X"}]}`, "Every item requires", 400},
		{`{"items":[{"provider":"openai","name":"X","apiKey":"x"},{"provider":"openai","name":"X","apiKey":"y"}]}`, "Duplicate provider/name in batch", 409},
		{`{"items":[{"provider":"custom-embedding-missing","name":"X","apiKey":"x"}]}`, "Custom Embedding node not found", 404},
		{`{"items":[{"provider":" OpenAI ","name":" Bulk ","apiKey":" secret ","priority":2.5}]}`, `"updated":false`, 201},
		{`{"items":[{"provider":"openai","name":"Bulk","apiKey":"new-secret"}]}`, `"updated":true`, 201},
	} {
		response := httptest.NewRecorder()
		r.ServeHTTP(response, httptest.NewRequest("POST", "/api/providers/bulk", strings.NewReader(tt.body)))
		if response.Code != tt.status || !strings.Contains(response.Body.String(), tt.needle) {
			t.Fatalf("bulk: %d %s", response.Code, response.Body.String())
		}
		if strings.Contains(response.Body.String(), "secret") {
			t.Fatal("bulk response leaked credential")
		}
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/providers", strings.NewReader(`{}`)))
	if w.Code != 401 {
		t.Fatalf("guard: %d", w.Code)
	}
}

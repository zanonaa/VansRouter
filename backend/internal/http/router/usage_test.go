package router

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestUsageEndpoints(t *testing.T) {
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
	// Dados fictícios: um registro por dia nos últimos 3 dias + providers distintos em requestDetails.
	for i := 0; i < 3; i++ {
		day := nowUTC().AddDate(0, 0, -i).Format("2006-01-02")
		if _, err = db.Exec(`INSERT INTO usageHistory(timestamp,provider,model,connectionId,apiKey,endpoint,promptTokens,completionTokens,cost,status,tokens,meta) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, day+"T12:00:00.000Z", "openai", "gpt", "c1", "k", "/v1/chat", 100, 50, 0.25, "ok", "{}", "{}"); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`INSERT INTO usageDaily(dateKey,data) VALUES(?,?)`, day, `{"promptTokens":100,"completionTokens":50,"cost":0.25,"requests":1}`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`INSERT INTO requestDetails(id,timestamp,provider,model,connectionId,status,data) VALUES('d1','2026-01-01T00:00:00Z','agentrouter',NULL,NULL,NULL,'{}'),('d2','2026-01-02T00:00:00Z','agentrouter',NULL,NULL,NULL,'{}'),('d3','2026-01-03T00:00:00Z','openai',NULL,NULL,NULL,'{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err = (database.Nodes{DB: db}).Create(ctx, map[string]any{"id": "agentrouter", "type": "openai-compatible", "name": "Meu Nó", "prefix": "a", "baseUrl": "https://x/v1"}); err != nil {
		t.Fatal(err)
	}
	authentication, _ := wiredAuth(t)
	api := &handlers.Providers{Usage: &handlers.UsageEndpoints{Usage: database.Usage{DB: db}, Nodes: database.Nodes{DB: db}}}
	r := NewWithNodes(nil, authentication, nil, api)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/usage/history", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"apiKeyMasked":"k***"`) {
		t.Fatalf("history: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/usage/history?provider=other", nil))
	if !strings.Contains(w.Body.String(), `[]`) {
		t.Fatalf("filter: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/usage/chart?period=bad", nil))
	if w.Code != 400 {
		t.Fatal("period")
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/usage/chart?period=all", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"requests":1`) {
		t.Fatalf("chart: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/usage/providers", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"name":"Meu Nó"`) {
		t.Fatalf("providers: %d %s", w.Code, w.Body.String())
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/usage/history", nil))
	if w.Code != 401 {
		t.Fatal("unguarded")
	}
}

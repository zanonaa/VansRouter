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

func TestLogsAndDetails(t *testing.T) {
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
	_, err = db.Exec(`INSERT INTO requestDetails(id,timestamp,provider,data) VALUES('a','2026-01-01T00:00:00.000Z','openai','{"id":"json-id","request":{"secret":"dummy"},"response":null,"extra":true}'),('b','2026-01-02T00:00:00.000Z','other','{"id":"b","model":"m"}'); INSERT INTO usageHistory(timestamp,provider,model,connectionId,promptTokens,completionTokens,status,tokens) VALUES('2026-01-01T00:00:00Z',NULL,NULL,'connection-id',NULL,NULL,NULL,'{"prompt_tokens":5,"completion_tokens":7}')`)
	if err != nil {
		t.Fatal(err)
	}
	authentication, _ := wiredAuth(t)
	r := NewWithNodes(nil, authentication, nil, &handlers.Providers{Logs: &handlers.LogEndpoints{Store: database.Usage{DB: db}, Connections: database.Connections{DB: db}}})
	for _, tt := range []struct {
		path   string
		status int
		needle string
	}{
		{"/api/usage/request-details?page=0", 400, "Page must be"}, {"/api/usage/request-details?pageSize=101", 400, "PageSize must be between 1 and 100"},
		{"/api/usage/request-details?provider=openai&endDate=2026-01-01", 200, `"redacted":true`},
		{"/api/usage/request-details/a", 200, `"secret":"dummy"`}, {"/api/usage/request-details/missing", 404, "Not found"},
		{"/api/usage/logs", 200, "connecti"}, {"/api/usage/request-logs", 200, " | 5 | 7 | -"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", tt.path, nil))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("%s: %d %s", tt.path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/usage/request-details?provider=openai&page=1junk", nil))
	var result map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &result)
	page := result["pagination"].(map[string]any)
	if page["totalItems"] != float64(1) || page["page"] != float64(1) {
		t.Fatal(page)
	}
	if strings.Contains(w.Body.String(), "dummy") || !strings.Contains(w.Body.String(), "json-id") {
		t.Fatal("redaction or JSON identity changed")
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/usage/request-details/a", nil))
	if w.Code != 401 {
		t.Fatal("unguarded detail")
	}
}

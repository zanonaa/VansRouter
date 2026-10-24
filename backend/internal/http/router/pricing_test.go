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

func TestPricing(t *testing.T) {
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
	auth, _ := wiredAuth(t)
	r := NewWithNodes(nil, auth, nil, &handlers.Providers{Pricing: &handlers.Pricing{Store: database.Pricing{DB: db}}})
	for _, tt := range []struct{ body, needle string }{
		{`null`, "Invalid pricing data format"}, {`{"p":3}`, "Invalid pricing for provider: p"}, {`{"p":{"m":null}}`, "Invalid pricing for model: p/m"}, {`{"p":{"m":{"bad":1}}}`, "Invalid pricing field: bad"}, {`{"p":{"m":{"input":-1}}}`, "must be non-negative number"},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/pricing", strings.NewReader(tt.body)))
		if w.Code != 400 || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	for _, body := range []string{`{"custom":{"m":{"input":2,"output":3}}}`, `{"custom":{"n":{"input":4}}}`} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("PATCH", "/api/pricing", strings.NewReader(body)))
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/pricing", nil))
	var pricing map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &pricing)
	custom := pricing["custom"].(map[string]any)
	if len(custom) != 2 {
		t.Fatal(custom)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/pricing?provider=custom&model=m", nil))
	user, err := (database.Pricing{DB: db}).User(ctx)
	if err != nil || len(user["custom"].(map[string]any)) != 1 {
		t.Fatalf("reset model: %v %v", user, err)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/pricing", nil))
	user, err = (database.Pricing{DB: db}).User(ctx)
	if err != nil || len(user) != 0 {
		t.Fatal(user, err)
	}
	auth.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/pricing", nil))
	if w.Code != 401 {
		t.Fatal("unguarded")
	}
}

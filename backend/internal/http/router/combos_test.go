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

func TestCombosCRUD(t *testing.T) {
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
	authentication, _ := wiredAuth(t)
	api := &handlers.Providers{Combos: &handlers.Combos{Store: database.Combos{DB: db}}}
	r := NewWithNodes(nil, authentication, nil, api)
	for _, tt := range []struct {
		body, needle string
		status       int
	}{
		{`{}`, "Name is required", 400}, {`{"name":"bad space"}`, "Name can only contain", 400}, {`{"name":"x","context_length":0}`, "positive integer", 400}, {`{"name":"x","context_length":2000001}`, "must not exceed", 400},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/combos", strings.NewReader(tt.body)))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/combos", strings.NewReader(`{"name":"combo.one","models":["openai/model"],"context_length":"4096"}`)))
	if w.Code != 201 {
		t.Fatal(w.Body.String())
	}
	var combo map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &combo)
	id := combo["id"].(string)
	if combo["context_length"] != float64(4096) {
		t.Fatal(combo)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/combos", strings.NewReader(`{"name":"combo.one"}`)))
	if w.Code != 400 {
		t.Fatal("duplicate allowed")
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("PUT", "/api/combos/"+id, strings.NewReader(`{"name":"renamed","context_length":null,"models":[]}`)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	got, err := (database.Combos{DB: db}).Get(ctx, id)
	if err != nil || got["name"] != "renamed" || got["context_length"] != nil {
		t.Fatalf("update: %v %v", got, err)
	}
	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/combos/"+id, nil))
		if w.Code != 200 && w.Code != 204 {
			t.Fatalf("%s %d", method, w.Code)
		}
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/combos/"+id, nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/combos/"+id, nil))
	if w.Code != 404 {
		t.Fatal("deleted still exists")
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/combos", nil))
	if w.Code != 401 {
		t.Fatal("unguarded combos")
	}
}

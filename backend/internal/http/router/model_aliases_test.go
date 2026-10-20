package router

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestModelAliasesAndCustomModels(t *testing.T) {
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
	api := &handlers.Providers{Aliases: &handlers.ModelAliases{Store: database.ModelAliases{DB: db}}}
	r := NewWithNodes(nil, authentication, nil, api)
	for _, tt := range []struct {
		method, path, body, needle string
		status                     int
	}{
		{"PUT", "/api/models/alias", `{}`, "Model and alias required", 400},
		{"PUT", "/api/models/alias", `{"model":"openai/gpt","alias":"fast"}`, "success", 200},
		{"GET", "/api/models/alias", "", "fast", 200},
		{"DELETE", "/api/models/alias?alias=", "", "Alias required", 400},
		{"DELETE", "/api/models/alias?alias=fast", "", "success", 200},
		{"POST", "/api/models/custom", `{}`, "providerAlias and id required", 400},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("%s %s: %d %s", tt.method, tt.path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/models/custom", strings.NewReader(`{"providerAlias":"oa","id":"custom","caps":{"vision":true,"reasoning":"yes","other":false},"transport":"gemini-live"}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"added":true`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/models/custom", nil))
	if !strings.Contains(w.Body.String(), `"vision":true`) || strings.Contains(w.Body.String(), "other") || strings.Contains(w.Body.String(), "reasoning") || strings.Contains(w.Body.String(), "transport") {
		t.Fatalf("sanitize llm: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/models/custom", strings.NewReader(`{"providerAlias":"oa","id":"stt-model","type":"stt","transport":"gemini-live"}`)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/models/custom", nil))
	if !strings.Contains(w.Body.String(), `"transport":"gemini-live"`) {
		t.Fatalf("stt transport: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/models/custom", strings.NewReader(`{"providerAlias":"oa","id":"custom","name":"Updated","caps":{"reasoning":true}}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"added":false`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/models/custom", nil))
	if !strings.Contains(w.Body.String(), `"name":"Updated"`) || !strings.Contains(w.Body.String(), `"reasoning":true`) || strings.Contains(w.Body.String(), `"vision":true`) {
		t.Fatalf("upsert merge: %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/models/custom", strings.NewReader(`{"models":[{"providerAlias":"oa","id":"a"},{"providerAlias":"oa","id":"a"},{"id":"missing"},{"providerAlias":"oa","id":"b","name":"B"}]}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"count":2`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/api/models/custom?providerAlias=oa&id=a", nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/models/custom", nil))
	if strings.Contains(w.Body.String(), `"id":"a"`) || !strings.Contains(w.Body.String(), `"id":"b"`) {
		t.Fatalf("delete: %s", w.Body.String())
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/models/custom", nil))
	if w.Code != 401 {
		t.Fatal("unguarded")
	}
}

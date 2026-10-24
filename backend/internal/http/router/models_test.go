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

func TestModelsEndpoints(t *testing.T) {
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
	api := &handlers.Providers{Models: &handlers.ModelsEndpoints{Disabled: handlers.DisabledModels{DB: db}, Aliases: database.ModelAliases{DB: db}}, Aliases: &handlers.ModelAliases{Store: database.ModelAliases{DB: db}}}
	r := NewWithNodes(nil, authentication, nil, api)
	for _, tt := range []struct {
		method, path, body, needle string
		status                     int
	}{
		{"GET", "/api/models/disabled", "", `"disabled":{}`, 200},
		{"POST", "/api/models/disabled", `{}`, "providerAlias and ids[] required", 400},
		{"POST", "/api/models/disabled", `{"providerAlias":"oa","ids":["m1","m2"]}`, "success", 200},
		{"GET", "/api/models/disabled?providerAlias=oa", "", `"ids":["m1","m2"]`, 200},
		{"POST", "/api/models/disabled", `{"providerAlias":"oa","ids":["m1","m3"]}`, "success", 200},
		{"GET", "/api/models/disabled?providerAlias=oa", "", `"ids":["m1","m2","m3"]`, 200},
		{"DELETE", "/api/models/disabled?providerAlias=oa&id=m3", "", "success", 200},
		{"GET", "/api/models/disabled?providerAlias=other", "", `"ids":[]`, 200},
		{"DELETE", "/api/models/disabled", "", `"error":"providerAlias required"`, 400},
		{"DELETE", "/api/models/disabled?providerAlias=oa&id=m1", "", "success", 200},
		{"GET", "/api/models/disabled?providerAlias=oa", "", `"ids":["m2"]`, 200},
		{"PUT", "/api/models", `{}`, "Model and alias required", 400},
		{"PUT", "/api/models", `{"model":"agentrouter/claude-opus-4-6","alias":"taken"}`, "success", 200},
		{"PUT", "/api/models", `{"model":"openai/other","alias":"taken"}`, "Alias already in use", 400},
		{"PUT", "/api/models", `{"model":"openai/other","alias":"ok"}`, "success", 200},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body)))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.needle) {
			t.Fatalf("%s %s: %d %s", tt.method, tt.path, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/models", nil))
	if w.Code != 200 {
		t.Fatalf("models: %d %s", w.Code, w.Body.String()[:200])
	}
	var list struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.NewDecoder(w.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Models) == 0 {
		t.Fatal("empty catalog")
	}
	sawAlias := false
	for _, m := range list.Models {
		if alias, _ := m["alias"].(string); alias == "taken" {
			sawAlias = true
		}
	}
	if !sawAlias {
		t.Fatal("alias not applied in listing")
	}
	// Disabled filtering: disable a real catalog model id from provider "openai".
	first := list.Models[0]
	providerID, _ := first["provider"].(string)
	modelID, _ := first["model"].(string)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/api/models/disabled", strings.NewReader(`{"providerAlias":"`+providerID+`","ids":["`+modelID+`"]}`)))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/models", nil))
	var filtered struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.NewDecoder(w.Body).Decode(&filtered); err != nil {
		t.Fatal(err)
	}
	for _, m := range filtered.Models {
		if m["provider"] == providerID && m["model"] == modelID {
			t.Fatalf("disabled model %s/%s still listed", providerID, modelID)
		}
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/models", nil))
	if w.Code != 401 {
		t.Fatal("unguarded models")
	}
}

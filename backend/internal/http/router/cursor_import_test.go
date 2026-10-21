package router

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestCursorManualImport(t *testing.T) {
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
	store := database.Connections{DB: db}
	auth, _ := wiredAuth(t)
	r := NewWithNodes(nil, auth, nil, &handlers.Providers{OAuth: &handlers.OAuthEngine{CreateConnection: store.Create}})
	machine := "01234567-89ab-cdef-0123-456789abcdef"
	token := buildFakeJWT(map[string]any{"email": "op@example.test", "sub": "user"})
	for _, tt := range []struct {
		body, message string
		status        int
	}{{`{}`, "Access token is required", 400}, {`{"accessToken":"t"}`, "Machine ID is required", 400}, {`{"accessToken":"short","machineId":"` + machine + `"}`, "Token appears too short", 500}, {`{"accessToken":"` + token + `","machineId":"bad"}`, "Invalid machine ID format", 500}} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/cursor/import", strings.NewReader(tt.body)))
		if w.Code != tt.status || !strings.Contains(w.Body.String(), tt.message) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for i := 0; i < 2; i++ {
		payload, _ := json.Marshal(map[string]string{"accessToken": " " + token + " ", "machineId": " " + machine + " "})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/api/oauth/cursor/import", strings.NewReader(string(payload))))
		if w.Code != 200 || strings.Contains(w.Body.String(), token) || !strings.Contains(w.Body.String(), "op@example.test") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	list, err := store.List(ctx, "cursor", nil)
	if err != nil || len(list) != 1 {
		t.Fatal("machine dedup", len(list), err)
	}
	row := list[0]
	specific := row["providerSpecificData"].(map[string]any)
	if row["accessToken"] != token || specific["machineId"] != machine || specific["userId"] != "user" || specific["authMethod"] != "imported" {
		t.Fatal(row)
	}
	expiry, err := time.Parse(time.RFC3339Nano, row["expiresAt"].(string))
	if err != nil || time.Until(expiry) < 23*time.Hour {
		t.Fatal(row)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/oauth/cursor/import", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "state.vscdb") || !strings.Contains(w.Body.String(), "requiredFields") {
		t.Fatal(w.Code, w.Body.String())
	}
	auth.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/oauth/cursor/import", nil))
	if w.Code != 401 {
		t.Fatal("unguarded")
	}
}

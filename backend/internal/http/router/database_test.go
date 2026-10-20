package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestDatabaseExportImport(t *testing.T) {
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
	if _, err = db.Exec(`INSERT INTO settings(id,data) VALUES(1,'{"password":"hash-ficticio"}'); INSERT INTO kv(scope,key,value) VALUES('pricing','openai','{}')`); err != nil {
		t.Fatal(err)
	}
	validPassword := false
	handler := handlers.DatabaseExport{DB: db, VerifyPassword: func(password string) bool { return validPassword && password == "ficticia" }}
	authentication, _ := wiredAuth(t)
	r := NewWithNodes(nil, authentication, nil, &handlers.Providers{Database: &handler})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/settings/database", nil))
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	token, err := authentication.Sessions.Create(nil)
	if err != nil {
		t.Fatal(err)
	}
	sessionCookie := &http.Cookie{Name: "auth_token", Value: token}
	validPassword = true
	request := httptest.NewRequest("GET", "/api/settings/database", nil)
	request.Header.Set("X-9r-password", "ficticia")
	request.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload["settings"] == nil || payload["kv"] == nil {
		t.Fatal("export incomplete")
	}
	payload["kv"] = []any{map[string]any{"scope": "pricing", "key": "openai", "value": "{}"}}
	encoded, _ := json.Marshal(map[string]any{"password": "ficticia", "settings": payload["settings"], "kv": payload["kv"], "unknown_table": []any{}})
	post := httptest.NewRequest("POST", "/api/settings/database", strings.NewReader(string(encoded)))
	post.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, post)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "unknown table") {
		t.Fatal(w.Code, w.Body.String())
	}
	delete(payload, "unknown_table")
	encoded, _ = json.Marshal(map[string]any{"password": "ficticia", "settings": payload["settings"], "kv": payload["kv"]})
	post = httptest.NewRequest("POST", "/api/settings/database", strings.NewReader(string(encoded)))
	post.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, post)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM kv WHERE scope='pricing'`).Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/settings/database", nil))
	if w.Code != 401 {
		t.Fatal("unguarded")
	}
}

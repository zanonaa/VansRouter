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

func TestDatabaseLogicalTransfer(t *testing.T) {
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
	_, err = db.Exec(`INSERT INTO settings(id,data) VALUES(1,'{"password":"hash-ficticio"}'); INSERT INTO kv(scope,key,value) VALUES('pricing','custom','{"model":{"input":2}}'),('disabledModels','p','["m"]'); INSERT INTO requestDetails(id,timestamp,data) VALUES('detail','now','{}'); INSERT INTO usageHistory(timestamp,tokens) VALUES('now','{}'); CREATE TABLE user_work(id TEXT); INSERT INTO user_work VALUES('keep');`)
	if err != nil {
		t.Fatal(err)
	}
	authentication, _ := wiredAuth(t)
	token, _ := authentication.Sessions.Create(nil)
	cookie := &http.Cookie{Name: "auth_token", Value: token}
	handler := handlers.DatabaseExport{DB: db, ValidCLI: authentication.ValidCLIToken, VerifyPassword: func(value string) bool { return value == "dummy-password" }}
	r := NewWithNodes(nil, authentication, nil, &handlers.Providers{Database: &handler})
	request := func(method, body, password, cli string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/settings/database", strings.NewReader(body))
		req.AddCookie(cookie)
		req.Header.Set("X-9r-password", password)
		if cli != "" {
			req.Header.Set("X-9r-cli-token", cli)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := request("GET", "", "", "invalid-token"); w.Code != 401 {
		t.Fatal("invalid CLI header bypass", w.Code)
	}
	w := request("GET", "", "dummy-password", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var payload map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if _, ok := payload["settings"].(map[string]any); !ok {
		t.Fatal("settings not logical", payload["settings"])
	}
	if payload["pricing"] == nil || payload["kv"] != nil || payload["usageHistory"] != nil || payload["_meta"] != nil {
		t.Fatal("internal tables exported")
	}
	payload["password"] = "dummy-password"
	payload["settings"] = map[string]any{"theme": "new"}
	encoded, _ := json.Marshal(payload)
	w = request("POST", string(encoded), "", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, query := range []string{`SELECT count(*) FROM requestDetails`, `SELECT count(*) FROM usageHistory`, `SELECT count(*) FROM user_work`, `SELECT count(*) FROM kv WHERE scope='disabledModels'`} {
		var count int
		if err = db.QueryRow(query).Scan(&count); err != nil || count != 1 {
			t.Fatal("unrelated data lost", query, count, err)
		}
	}
	var revision string
	if err = db.QueryRow(`SELECT value FROM _meta WHERE key='settings_revision'`).Scan(&revision); err != nil || revision != "1" {
		t.Fatal(revision, err)
	}
	// Erro após o DELETE de settings deve reverter integralmente o import.
	w = request("POST", `{"password":"dummy-password","settings":{"theme":"bad"},"providerConnections":[{"id":"invalid"}]}`, "", "")
	if w.Code != 400 {
		t.Fatal(w.Code, w.Body.String())
	}
	settings, err := database.ReadAuthSettings(ctx, db)
	if err != nil || settings["theme"] != "new" {
		t.Fatal("rollback failed", settings, err)
	}
	w = request("GET", "", "", authentication.CLIToken())
	if w.Code != 200 {
		t.Fatal("valid CLI refused", w.Code)
	}
}

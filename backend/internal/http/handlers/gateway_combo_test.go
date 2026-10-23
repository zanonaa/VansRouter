package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestChatComboFallbackAndStickyRotation(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}'`)
	_, err := g.DB.Exec(`CREATE TABLE IF NOT EXISTS combos(id TEXT PRIMARY KEY,name TEXT,kind TEXT,models TEXT,context_length REAL,createdAt TEXT,updatedAt TEXT);INSERT INTO combos VALUES('combo','bundle','llm','["openai/first","openai/second"]',NULL,'now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	requests := []string{}
	failFirst := true
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		text := string(raw)
		model := "second"
		if strings.Contains(text, `"model":"first"`) {
			model = "first"
		}
		requests = append(requests, model)
		status := 200
		response := "data: success\n\n"
		if model == "first" && failFirst {
			status = 429
			response = "failed-candidate-must-not-leak"
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(response))}, nil
	})}}
	invoke := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"bundle","messages":[{"role":"user","content":"hello"}]} `)))
		return w
	}
	w := invoke()
	if w.Code != 200 || len(requests) != 2 || strings.Contains(w.Body.String(), "must-not-leak") {
		t.Fatalf("fallback %d %v %s", w.Code, requests, w.Body.String())
	}
	failFirst = false
	requests = nil
	providers.ResetComboRotation("bundle")
	defer providers.ResetComboRotation("bundle")
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false,"comboStrategy":"round-robin","comboStickyRoundRobinLimit":2}'`)
	for i := 0; i < 4; i++ {
		if w := invoke(); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	if strings.Join(requests, ",") != "first,first,second,second" {
		t.Fatal(requests)
	}
}
func TestComboPermissionBlocksBeforeCandidate(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`CREATE TABLE IF NOT EXISTS combos(id TEXT PRIMARY KEY,name TEXT,kind TEXT,models TEXT,context_length REAL,createdAt TEXT,updatedAt TEXT);INSERT INTO combos VALUES('c','bundle','llm','["openai/model"]',NULL,'now','now');`)
	key, err := database.CreateApiKey(context.Background(), g.DB, "combo", "machine")
	if err != nil {
		t.Fatal(err)
	}
	database.UpdateApiKey(context.Background(), g.DB, ttsString(key["id"]), map[string]any{"allowedCombos": []any{}})
	g.Send = &TranslatorSend{}
	r := httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"bundle","messages":[]}`))
	r.Header.Set("Authorization", "Bearer "+ttsString(key["key"]))
	w := httptest.NewRecorder()
	g.OpenAIChat(w, r)
	if w.Code != 403 || !strings.Contains(w.Body.String(), `Combo`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestComboListedIdIsRoutable(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}'`)
	if _, err := g.DB.Exec(`INSERT INTO combos VALUES('c','bundle','llm','["openai/model"]',NULL,'now','now');`); err != nil {
		t.Fatal(err)
	}
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
	})}}
	// "combo/bundle" é o id publicado pelo /v1/models e precisa rotear.
	w := httptest.NewRecorder()
	g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"combo/bundle","messages":[{"role":"user","content":"hi"}]}`)))
	if w.Code != 200 {
		t.Fatalf("id listado não roteia: %d %s", w.Code, w.Body.String())
	}
}

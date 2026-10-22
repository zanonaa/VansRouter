package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func TestOpenAIChatUsesResolvedProvider(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE apiKeys(id TEXT,key TEXT,isActive INTEGER); INSERT INTO apiKeys VALUES('k','secret',1); CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT); INSERT INTO providerConnections VALUES('c','openai','apikey','OpenAI',NULL,1,1,'{"apiKey":"upstream"}','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	send := &TranslatorSend{Connections: database.Connections{DB: db}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if !strings.Contains(r.URL.String(), "api.openai.com") || r.Header.Get("Authorization") != "Bearer upstream" {
			t.Fatalf("request %s %s", r.URL.String(), r.Header.Get("Authorization"))
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
	})}}
	gateway := &Gateway{DB: db, Send: send}
	request := httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
	request.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	gateway.OpenAIChat(w, request)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "data: ok") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	request = httptest.NewRequest("POST", "/", strings.NewReader(`{"model":"claude/claude","messages":[]}`))
	request.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	gateway.OpenAIChat(w, request)
	if w.Code != 501 {
		t.Fatalf("non-openai %d", w.Code)
	}
}

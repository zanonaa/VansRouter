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

func webGateway(t *testing.T, transport http.RoundTripper) *Gateway {
	t.Helper()
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE apiKeys(id TEXT,key TEXT,isActive INTEGER); INSERT INTO apiKeys VALUES('k','secret',1); CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT); INSERT INTO providerConnections VALUES('c','serper','apikey','Serper',NULL,1,1,'{"apiKey":"upstream"}','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	return &Gateway{DB: db, Client: &http.Client{Transport: transport}}
}

func TestSearchAndFetchContracts(t *testing.T) {
	gateway := webGateway(t, kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "google.serper.dev" || r.Header.Get("X-API-Key") != "upstream" {
			t.Fatalf("search request %s", r.URL.String())
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"organic":[]}`))}, nil
	}))
	request := httptest.NewRequest("POST", "/api/v1/search", strings.NewReader(`{"provider":"serper","query":"golang"}`))
	request.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	gateway.Search(w, request)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"provider":"serper"`) {
		t.Fatalf("search %d %s", w.Code, w.Body.String())
	}
	request = httptest.NewRequest("POST", "/api/v1/web/fetch", strings.NewReader(`{"provider":"firecrawl","url":"http://127.0.0.1/private"}`))
	request.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	gateway.Fetch(w, request)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "not public") {
		t.Fatalf("ssrf %d %s", w.Code, w.Body.String())
	}
}

package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func videoFixture(t *testing.T) *Gateway {
	t.Helper()
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE settings(id INTEGER PRIMARY KEY,data TEXT);INSERT INTO settings VALUES(1,'{"requireApiKey":false}');CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT);INSERT INTO providerConnections VALUES('first','xai','apikey',NULL,NULL,1,1,'{"apiKey":"wrong-account"}','now','now'),('pinned','xai','apikey',NULL,NULL,2,1,'{"apiKey":"correct-account"}','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	return &Gateway{DB: db}
}
func videoRequest(method, path, id string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	route := chi.NewRouteContext()
	route.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
}
func TestVideoPollingPinsCreatingAccount(t *testing.T) {
	g := videoFixture(t)
	calls := 0
	g.Client = &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "Bearer correct-account" || r.URL.Path != "/v1/videos/job" {
			t.Fatalf("wrong account/URL: %s %s", r.Header.Get("Authorization"), r.URL)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"status":"pending"}`))}, nil
	})}
	r := videoRequest("GET", "/api/v1/videos/job", "job")
	w := httptest.NewRecorder()
	g.Video(w, r)
	if w.Code != 400 || calls != 0 {
		t.Fatalf("missing pin %d calls %d", w.Code, calls)
	}
	r = videoRequest("GET", "/api/v1/videos/job", "job")
	r.Header.Set("x-connection-id", "pinned")
	w = httptest.NewRecorder()
	g.Video(w, r)
	if w.Code != 200 || calls != 1 || w.Header().Get("x-9router-connection-id") != "pinned" {
		t.Fatalf("poll %d %s", w.Code, w.Body.String())
	}
	r = videoRequest("GET", "/api/v1/videos/job?provider=openrouter", "job")
	r.Header.Set("x-connection-id", "pinned")
	w = httptest.NewRecorder()
	g.Video(w, r)
	if w.Code != 400 || calls != 1 {
		t.Fatal("provider mismatch sent upstream")
	}
}
func TestVideoCreationPreservesMultipartAndNeverRetries500(t *testing.T) {
	g := videoFixture(t)
	payload := "--original-boundary\r\nraw multipart bytes\r\n--original-boundary--"
	calls := 0
	g.Client = &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body, _ := io.ReadAll(r.Body)
		if string(body) != payload || r.Header.Get("Content-Type") != "multipart/form-data; boundary=original-boundary" || r.Header.Get("Idempotency-Key") != "job-key" {
			t.Fatal("creation body/headers modified")
		}
		return &http.Response{StatusCode: 500, Body: io.NopCloser(strings.NewReader(`{"error":"upstream"}`))}, nil
	})}
	r := httptest.NewRequest("POST", "/api/v1/videos/generations", strings.NewReader(payload))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=original-boundary")
	r.Header.Set("Idempotency-Key", "job-key")
	w := httptest.NewRecorder()
	g.Video(w, r)
	if w.Code != 500 || calls != 1 {
		t.Fatalf("billable request retried: status %d calls %d", w.Code, calls)
	}
}

package router

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestTags(t *testing.T) {
	auth, _ := wiredAuth(t)
	r := NewWithAuth(nil, auth)
	for _, method := range []string{"GET", "HEAD", "OPTIONS"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(method, "/api/tags", nil))
		if w.Code != 200 || w.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("%s: %d", method, w.Code)
		}
		if method == "GET" {
			var payload struct {
				Models []map[string]any `json:"models"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil || len(payload.Models) != 2 {
				t.Fatal(w.Body.String(), err)
			}
		} else if w.Body.Len() != 0 {
			t.Fatal("unexpected body")
		}
	}
	auth.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/tags", nil))
	if w.Code != 401 {
		t.Fatal("tags auth policy changed")
	}
}

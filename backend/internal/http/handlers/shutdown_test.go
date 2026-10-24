package handlers

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestShutdownPolicy(t *testing.T) {
	t.Setenv("SHUTDOWN_SECRET", "dummy")
	t.Setenv("NODE_ENV", "production")
	requested := make(chan struct{}, 1)
	h := Shutdown{Request: func() { requested <- struct{}{} }}
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Authorization", "Bearer dummy")
	w := httptest.NewRecorder()
	h.Post(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
	t.Setenv("NODE_ENV", "development")
	r.Header.Del("Authorization")
	w = httptest.NewRecorder()
	h.Post(w, r)
	if w.Code != 401 {
		t.Fatal(w.Code)
	}
	r.Header.Set("Authorization", "Bearer dummy")
	w = httptest.NewRecorder()
	h.Post(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	select {
	case <-requested:
		t.Fatal("shutdown before response delay")
	default:
	}
	select {
	case <-requested:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown not requested")
	}
}

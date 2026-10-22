package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestProviderModelTestsWarmupAndResults(t *testing.T) {
	store := &gcpStore{item: map[string]any{"provider": "openai-compatible-test"}}
	var mu sync.Mutex
	calls := 0
	firstCompleted := false
	ping := &ModelPing{BaseURL: "http://127.0.0.1:20128", Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"models":[{"id":"first"},{"id":"second"}]}`))}, nil
		}
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 2 && !firstCompleted {
			t.Fatal("parallel calls started before warmup")
		}
		firstCompleted = true
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`))}, nil
	})}}
	handler := &ProviderModelTests{Connections: store, Ping: ping}
	w := httptest.NewRecorder()
	handler.Post(w, httptest.NewRequest("POST", "/", nil))
	if w.Code != 200 || calls != 2 || !strings.Contains(w.Body.String(), `"modelId":"second"`) {
		t.Fatalf("status %d calls %d body %s", w.Code, calls, w.Body.String())
	}
}

package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestKiloFreeModelsCacheAndStale(t *testing.T) {
	calls := 0
	k := &KiloFreeModels{Client: &http.Client{Transport: kiroImportTransport(func(*http.Request) (*http.Response, error) {
		calls++
		if calls > 1 {
			return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"free","name":"Free","isFree":true},{"id":"paid","isFree":false}]}`))}, nil
	})}}
	w := httptest.NewRecorder()
	k.Get(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "paid") || !strings.Contains(w.Body.String(), `"cached":false`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	k.Get(w, httptest.NewRequest("GET", "/", nil))
	if calls != 1 || !strings.Contains(w.Body.String(), `"cached":true`) {
		t.Fatal("cache miss")
	}
	k.cachedAt = time.Now().Add(-2 * time.Hour)
	w = httptest.NewRecorder()
	k.Get(w, httptest.NewRequest("GET", "/", nil))
	if !strings.Contains(w.Body.String(), `"warning":"Kilo API returned 503"`) {
		t.Fatal(w.Body.String())
	}
}

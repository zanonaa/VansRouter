package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderValidationStatusAndUnsupported(t *testing.T) {
	h := &ProviderValidation{Client: &http.Client{Transport: kiroImportTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}}
	w := httptest.NewRecorder()
	h.Post(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"provider":"openai","apiKey":"key"}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"valid":true`) {
		t.Fatal(w.Body.String())
	}
	w = httptest.NewRecorder()
	h.Post(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"provider":"not-ported","apiKey":"key"}`)))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "validation unavailable") {
		t.Fatalf("unsupported status %d", w.Code)
	}
}

package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type sendConnections struct{ items []map[string]any }

func (s sendConnections) List(context.Context, string, *bool) ([]map[string]any, error) {
	return s.items, nil
}
func (s sendConnections) Update(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, nil
}
func (s sendConnections) DeleteByProvider(context.Context, string) (int64, error) { return 0, nil }
func TestTranslatorSendStreamsOpenAICompatible(t *testing.T) {
	handler := &TranslatorSend{Connections: sendConnections{[]map[string]any{{"provider": "openai-compatible-test", "isActive": true, "apiKey": "secret", "providerSpecificData": map[string]any{"baseUrl": "https://example.invalid/v1"}}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://example.invalid/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatal("wrong upstream request")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: chunk\n\n"))}, nil
	})}}
	w := httptest.NewRecorder()
	handler.Send(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"provider":"openai-compatible-test","model":"model","body":{"messages":[]}}`)))
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/event-stream" || !strings.Contains(w.Body.String(), "data: chunk") {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	handler.Send(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"provider":"special","model":"model","body":{}}`)))
	if w.Code != 501 {
		t.Fatalf("unsupported status %d", w.Code)
	}
}

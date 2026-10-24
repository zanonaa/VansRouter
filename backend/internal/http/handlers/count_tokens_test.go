package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCountTokensMatchesAnthropicEstimate(t *testing.T) {
	gateway := &Gateway{}
	w := httptest.NewRecorder()
	gateway.CountTokens(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"system":"abcd","messages":[{"content":[{"type":"text","text":"efgh"}]}]}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"input_tokens":2`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	gateway.CountTokens(w, httptest.NewRequest("POST", "/", strings.NewReader(`{`)))
	if w.Code != 400 || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("invalid %d", w.Code)
	}
}

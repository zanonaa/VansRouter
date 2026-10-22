package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranslatorDetectsStepOne(t *testing.T) {
	handler := &TranslatorTranslate{}
	w := httptest.NewRecorder()
	handler.Post(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"step":1,"body":{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}]}}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"sourceFormat":"openai"`) || !strings.Contains(w.Body.String(), `"targetFormat":"openai"`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	handler.Post(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"step":2,"body":{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}]}}`)))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"body"`) {
		t.Fatalf("step2 %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	handler.Post(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"step":4,"body":{}}`)))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

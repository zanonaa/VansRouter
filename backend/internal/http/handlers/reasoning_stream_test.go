package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiThoughtStreamsAsReasoning(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai", target: "gemini", model: "model"}
	frames := []string{`{"candidates":[{"content":{"parts":[{"thought":true,"text":"plan"},{"text":"answer"}]}}]}`, `{"candidates":[{"finishReason":"STOP"}]}`}
	for _, frame := range frames {
		if _, err := s.Write([]byte("data: " + frame + "\n\n")); err != nil {
			t.Fatal(err)
		}
	}
	if !strings.Contains(w.Body.String(), `"reasoning_content":"plan"`) || !strings.Contains(w.Body.String(), `"content":"answer"`) {
		t.Fatal(w.Body.String())
	}
}

func TestResponsesReasoningSummaryLifecycle(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai-responses", target: "openai", model: "model"}
	frames := []string{`{"id":"reply","choices":[{"index":0,"delta":{"reasoning_content":"think"}}]}`, `{"choices":[{"index":0,"delta":{"content":"answer"},"finish_reason":"stop"}]}`}
	for _, frame := range frames {
		if _, err := s.Write([]byte("data: " + frame + "\n\n")); err != nil {
			t.Fatal(err)
		}
	}
	s.Write([]byte("data: [DONE]\n\n"))
	s.finish()
	body := w.Body.String()
	for _, needle := range []string{"event: response.reasoning_summary_part.added", "event: response.reasoning_summary_text.delta", "event: response.reasoning_summary_text.done", "event: response.output_item.done", `"type":"reasoning"`} {
		if !strings.Contains(body, needle) {
			t.Fatalf("lost %s: %s", needle, body)
		}
	}
	if strings.Index(body, "reasoning_summary_text.done") > strings.Index(body, "output_text.delta") {
		t.Fatal("reasoning closed after visible output")
	}
}

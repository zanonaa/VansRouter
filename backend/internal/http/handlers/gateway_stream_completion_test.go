package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTruncatedProviderStreamNeverEmitsSuccessSentinel(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai", target: "claude"}
	s.Write([]byte("data: {\"type\":\"message_start\",\"message\":{\"id\":\"reply\"}}\n\n"))
	s.finish()
	if strings.Contains(w.Body.String(), "[DONE]") || !strings.Contains(w.Body.String(), "closed before completion") {
		t.Fatal(w.Body.String())
	}
}
func TestClaudeToolIndexDoesNotIncludeTextBlocks(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai", target: "claude"}
	frames := []string{`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`, `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"call","name":"lookup"}}`, `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{}"}}`, `{"type":"message_stop"}`}
	for _, frame := range frames {
		if _, err := s.Write([]byte("data: " + frame + "\n\n")); err != nil {
			t.Fatal(err)
		}
	}
	s.finish()
	if strings.Contains(w.Body.String(), `"index":1`) || !strings.Contains(w.Body.String(), `"tool_calls":[{"function"`) {
		t.Fatal(w.Body.String())
	}
}
func TestProviderErrorDoesNotBecomeDone(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai", target: "gemini"}
	s.Write([]byte("data: {\"error\":{\"message\":\"failed\"}}\n\n"))
	s.finish()
	if strings.Contains(w.Body.String(), "[DONE]") || !strings.Contains(w.Body.String(), "stream_conversion_error") {
		t.Fatal(w.Body.String())
	}
}

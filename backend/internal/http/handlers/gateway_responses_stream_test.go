package handlers

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponsesCompletionWaitsForUsageAndIncludesOutput(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai-responses", target: "openai", model: "model"}
	frames := []string{`{"id":"reply","choices":[{"index":0,"delta":{"role":"assistant","content":"hello "}}]}`, `{"id":"reply","choices":[{"index":0,"delta":{"content":"world"},"finish_reason":"stop"}]}`, `{"id":"reply","choices":[],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`}
	for _, frame := range frames {
		if _, err := s.Write([]byte("data: " + frame + "\n\n")); err != nil {
			t.Fatal(err)
		}
	}
	if strings.Contains(w.Body.String(), "event: response.completed") {
		t.Fatal("completed before usage trailer/DONE")
	}
	s.Write([]byte("data: [DONE]\n\n"))
	s.finish()
	for _, needle := range []string{"event: response.created", "event: response.output_text.delta", "event: response.output_item.done", "event: response.completed", `"text":"hello world"`, `"total_tokens":12`} {
		if !strings.Contains(w.Body.String(), needle) {
			t.Fatalf("lost %s: %s", needle, w.Body.String())
		}
	}
	sequence := -1
	for _, line := range strings.Split(w.Body.String(), "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			t.Fatal(err)
		}
		next := int(event["sequence_number"].(float64))
		if next != sequence+1 {
			t.Fatal("noncontiguous sequence")
		}
		sequence = next
	}
}
func TestResponsesToolsAndIncompleteCompletion(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai-responses", target: "openai", model: "model"}
	frames := []string{`{"id":"reply","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call","function":{"name":"lookup","arguments":"{"}}]}}]}`, `{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"q\":1}"}}]},"finish_reason":"length"}]}`}
	for _, frame := range frames {
		if _, err := s.Write([]byte("data: " + frame + "\n\n")); err != nil {
			t.Fatal(err)
		}
	}
	s.Write([]byte("data: [DONE]\n\n"))
	s.finish()
	for _, needle := range []string{"event: response.function_call_arguments.delta", "event: response.function_call_arguments.done", "event: response.incomplete", `"call_id":"call"`, `"reason":"max_output_tokens"`} {
		if !strings.Contains(w.Body.String(), needle) {
			t.Fatalf("lost %s: %s", needle, w.Body.String())
		}
	}
}

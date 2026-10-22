package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaudeSSEConvertsAcrossArbitraryChunks(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai", target: "claude", model: "model"}
	input := `data: {"type":"message_start","message":{"id":"reply"}}` + "\r\n\r\n" + `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"answer"}}` + "\n\n" + `data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}` + "\n\n" + `data: {"type":"message_stop"}` + "\n\n"
	for i := 0; i < len(input); i += 3 {
		end := i + 3
		if end > len(input) {
			end = len(input)
		}
		if _, err := s.Write([]byte(input[i:end])); err != nil {
			t.Fatal(err)
		}
	}
	s.finish()
	for _, needle := range []string{`"object":"chat.completion.chunk"`, `"content":"answer"`, `"finish_reason":"stop"`, "data: [DONE]"} {
		if !strings.Contains(w.Body.String(), needle) {
			t.Fatalf("lost %s: %s", needle, w.Body.String())
		}
	}
	if strings.Count(w.Body.String(), "[DONE]") != 1 {
		t.Fatal("duplicate completion sentinel")
	}
}
func TestOpenAISSEConvertsToolArgumentsIncrementally(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "claude", target: "openai", model: "model", textBlock: -1}
	frames := []string{`{"id":"reply","choices":[{"delta":{"role":"assistant"},"finish_reason":null}]}`, `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call","function":{"name":"lookup","arguments":"{"}}]},"finish_reason":null}]}`, `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"q\":1}"}}]},"finish_reason":null}]}`, `{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`}
	for _, frame := range frames {
		if _, err := s.Write([]byte("data: " + frame + "\n\n")); err != nil {
			t.Fatal(err)
		}
	}
	s.Write([]byte("data: [DONE]\n\n"))
	s.finish()
	for _, needle := range []string{"event: message_start", "event: content_block_start", `"name":"lookup"`, `"type":"input_json_delta"`, `"stop_reason":"tool_use"`, "event: message_stop"} {
		if !strings.Contains(w.Body.String(), needle) {
			t.Fatalf("lost %s: %s", needle, w.Body.String())
		}
	}
}
func TestMalformedStreamFailsExplicitly(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai", target: "claude"}
	if _, err := s.Write([]byte("data: invalid\n\n")); err == nil {
		t.Fatal("invalid JSON accepted")
	}
	s.finish()
	if !strings.Contains(w.Body.String(), "stream_conversion_error") {
		t.Fatal(w.Body.String())
	}
}

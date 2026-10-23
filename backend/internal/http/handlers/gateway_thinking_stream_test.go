package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// GLM (e outros providers Claude-compatíveis) emite blocos "thinking" no SSE.
// A conversão para OpenAI deve mandar o texto em reasoning_content e ignorar
// signature_delta — antes o gateway abortava com
// "Unsupported Claude streaming block: thinking" no meio do stream.
func TestClaudeThinkingStreamConvertsToReasoningContent(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai", target: "claude", model: "glm-5.3", serverTool: -1}
	frames := []string{
		`{"type":"message_start","message":{"id":"msg_1","model":"glm-5.3","usage":{"input_tokens":3}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"vou pensar"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"abc"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"ok"}}`,
		`{"type":"content_block_stop","index":1}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		`{"type":"message_stop"}`,
	}
	for _, frame := range frames {
		if _, err := s.Write([]byte("event: x\ndata: " + frame + "\n\n")); err != nil {
			t.Fatalf("frame %s: %v", frame, err)
		}
	}
	s.finish()
	body := w.Body.String()
	for _, needle := range []string{`"reasoning_content":"vou pensar"`, `"content":"ok"`, `"finish_reason":"stop"`, "data: [DONE]"} {
		if !strings.Contains(body, needle) {
			t.Fatalf("perdeu %s: %s", needle, body)
		}
	}
	if strings.Contains(body, "stream_conversion_error") {
		t.Fatalf("stream abortou: %s", body)
	}
}

// server_tool_use (web search) é executado no provider e não vira tool_call.
func TestClaudeServerToolUseIsSkipped(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai", target: "claude", model: "glm-5.3", serverTool: -1}
	frames := []string{
		`{"type":"message_start","message":{"id":"msg_2","model":"glm-5.3"}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"server_tool_use","id":"srv","name":"web_search"}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"query\":\"x\"}"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"resposta"}}`,
		`{"type":"message_stop"}`,
	}
	for _, frame := range frames {
		if _, err := s.Write([]byte("data: " + frame + "\n\n")); err != nil {
			t.Fatalf("frame %s: %v", frame, err)
		}
	}
	s.finish()
	body := w.Body.String()
	if strings.Contains(body, "tool_calls") || strings.Contains(body, "stream_conversion_error") {
		t.Fatalf("server tool vazou: %s", body)
	}
	if !strings.Contains(body, `"content":"resposta"`) {
		t.Fatalf("texto perdido: %s", body)
	}
}

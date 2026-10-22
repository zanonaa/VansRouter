package handlers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClaudeSourcePreservesToolsAndImages(t *testing.T) {
	var body map[string]any
	json.Unmarshal([]byte(`{"model":"model","system":"instructions","max_tokens":456,"messages":[{"role":"user","content":[{"type":"text","text":"question"},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"aGVsbG8="}}]},{"role":"assistant","content":[{"type":"tool_use","id":"tool-1","name":"lookup","input":{"q":"find"}}]},{"role":"user","content":[{"type":"tool_result","tool_use_id":"tool-1","content":"result"}]}],"tools":[{"name":"lookup","description":"search","input_schema":{"type":"object"}}]}`), &body)
	converted, err := translateSource(body, "claude")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(converted)
	for _, needle := range []string{"instructions", "question", "data:image/png;base64,aGVsbG8=", "tool-1", "lookup", "result", "456"} {
		if !strings.Contains(string(raw), needle) {
			t.Fatalf("lost %s: %s", needle, raw)
		}
	}
	messages := converted["messages"].([]any)
	if len(messages) != 4 || messages[3].(map[string]any)["role"] != "tool" {
		t.Fatalf("bad turns %s", raw)
	}
	if body["messages"].([]any)[0].(map[string]any)["content"].([]any)[1].(map[string]any)["type"] != "image" {
		t.Fatal("mutated source")
	}
}
func TestGeminiSourcePreservesTurnsAndGeneration(t *testing.T) {
	var body map[string]any
	json.Unmarshal([]byte(`{"systemInstruction":{"parts":[{"text":"rules"}]},"contents":[{"role":"user","parts":[{"text":"question"}]},{"role":"model","parts":[{"text":"answer"},{"functionCall":{"id":"call-1","name":"lookup","args":{"q":"test"}}}]},{"role":"user","parts":[{"functionResponse":{"id":"call-1","name":"lookup","response":{"value":"result"}}}]}],"generationConfig":{"maxOutputTokens":567,"temperature":0.1}}`), &body)
	out, err := translateSource(body, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(out)
	for _, needle := range []string{"rules", "question", "answer", "call-1", "lookup", "result", "567"} {
		if !strings.Contains(string(raw), needle) {
			t.Fatalf("lost %s: %s", needle, raw)
		}
	}
	if out["max_tokens"] != float64(567) {
		t.Fatal("generation config lost")
	}
}
func TestResponsesSourceTextAndToolTurns(t *testing.T) {
	var body map[string]any
	json.Unmarshal([]byte(`{"model":"model","instructions":"rules","input":[{"role":"user","content":[{"type":"input_text","text":"question"}]},{"type":"function_call","call_id":"call-1","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call-1","output":"result"}],"max_output_tokens":789}`), &body)
	out, err := translateSource(body, "openai-responses")
	if err != nil {
		t.Fatal(err)
	}
	if len(out["messages"].([]any)) != 4 || out["max_tokens"] != float64(789) {
		t.Fatalf("bad result %v", out)
	}
}
func TestSourceUnknownBlocksAreNotSilentlyDropped(t *testing.T) {
	_, err := translateSource(map[string]any{"messages": []any{map[string]any{"role": "user", "content": []any{map[string]any{"type": "unknown", "text": "must remain"}}}}}, "claude")
	if err == nil {
		t.Fatal("unknown block silently dropped")
	}
}

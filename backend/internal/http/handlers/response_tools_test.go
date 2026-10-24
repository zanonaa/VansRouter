package handlers

import "testing"

func TestResponsesToolOutputKeepsJSONArguments(t *testing.T) {
	native := map[string]any{"id": "reply", "choices": []any{map[string]any{"message": map[string]any{"content": "using tool", "tool_calls": []any{map[string]any{"id": "call", "type": "function", "function": map[string]any{"name": "lookup", "arguments": `{"q":1}`}}}}, "finish_reason": "tool_calls"}}}
	out, err := convertChatResponse(native, "openai", "openai-responses", "model")
	if err != nil {
		t.Fatal(err)
	}
	output := out["output"].([]any)
	call := output[1].(map[string]any)
	if output[0].(map[string]any)["type"] != "message" || call["type"] != "function_call" || call["call_id"] != "call" || call["arguments"] != `{"q":1}` {
		t.Fatal(out)
	}
	native["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)["tool_calls"].([]any)[0].(map[string]any)["function"].(map[string]any)["arguments"] = "{"
	if _, err = convertChatResponse(native, "openai", "openai-responses", "model"); err == nil {
		t.Fatal("accepted invalid arguments")
	}
}

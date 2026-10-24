package handlers

import "testing"

func TestGeminiToolChoiceModes(t *testing.T) {
	for _, test := range []struct {
		choice any
		mode   string
		name   string
	}{
		{"auto", "AUTO", ""},
		{"required", "ANY", ""},
		{map[string]any{"type": "function", "function": map[string]any{"name": "lookup"}}, "ANY", "lookup"},
	} {
		body, err := translateTarget(map[string]any{"messages": []any{map[string]any{"role": "user", "content": "hello"}}, "tool_choice": test.choice}, "gemini", "model")
		if err != nil {
			t.Fatal(err)
		}
		config := body["toolConfig"].(map[string]any)["functionCallingConfig"].(map[string]any)
		if config["mode"] != test.mode {
			t.Fatal(config)
		}
		if test.name != "" && config["allowedFunctionNames"].([]any)[0] != test.name {
			t.Fatal(config)
		}
	}
	if _, err := translateTarget(map[string]any{"messages": []any{map[string]any{"role": "user", "content": "hello"}}, "tool_choice": "forced"}, "gemini", "model"); err == nil {
		t.Fatal("accepted unsupported choice")
	}
}

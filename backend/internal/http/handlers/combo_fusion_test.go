package handlers

import (
	"strings"
	"testing"
)

func TestFusionExtractsAndFlattensPanel(t *testing.T) {
	if fusionText(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": " answer "}}}}) != "answer" {
		t.Fatal("openai text")
	}
	messages := flattenFusionHistory([]any{map[string]any{"role": "assistant", "content": "base", "tool_calls": []any{map[string]any{"function": map[string]any{"name": "lookup"}}}}, map[string]any{"role": "tool", "content": "result"}})
	if !strings.Contains(ttsString(messages[0].(map[string]any)["content"]), "[Called tools: lookup]") || messages[0].(map[string]any)["tool_calls"] != nil || !strings.Contains(ttsString(messages[1].(map[string]any)["content"]), "[Tool result: result]") {
		t.Fatal(messages)
	}
	prompt := fusionPrompt([]fusionAnswer{{text: "one"}, {text: "two"}})
	if !strings.Contains(prompt, "[Source 1]\none") || !strings.Contains(prompt, "Do NOT mention") {
		t.Fatal(prompt)
	}
}

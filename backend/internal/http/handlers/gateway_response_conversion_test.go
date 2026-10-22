package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGatewayConvertsClaudeRequestAndOpenAIResponse(t *testing.T) {
	g := gatewayModelsFixture(t)
	if _, err := g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}'`); err != nil {
		t.Fatal(err)
	}
	calls := 0
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		messages := body["messages"].([]any)
		if messages[0].(map[string]any)["role"] != "system" || body["max_tokens"] != float64(321) {
			t.Fatalf("request not converted %v", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":"reply","choices":[{"message":{"role":"assistant","content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":12,"completion_tokens":3,"total_tokens":15}}`))}, nil
	})}}
	request := httptest.NewRequest("POST", "/api/v1/messages", strings.NewReader(`{"model":"openai/gpt-4o","stream":false,"system":"instructions","messages":[{"role":"user","content":"question"}],"max_tokens":321}`))
	w := httptest.NewRecorder()
	g.OpenAIChat(w, request)
	if w.Code != 200 || calls != 1 || !strings.Contains(w.Body.String(), `"type":"message"`) || !strings.Contains(w.Body.String(), `"input_tokens":12`) {
		t.Fatalf("%d calls %d %s", w.Code, calls, w.Body.String())
	}
}
func TestConvertClaudeResponsePreservesToolCallsAndUsage(t *testing.T) {
	var native map[string]any
	json.Unmarshal([]byte(`{"id":"native","content":[{"type":"text","text":"answer"},{"type":"tool_use","id":"call","name":"lookup","input":{"q":"value"}}],"stop_reason":"tool_use","usage":{"input_tokens":10,"output_tokens":4}}`), &native)
	out, err := convertChatResponse(native, "claude", "openai", "model")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(out)
	for _, needle := range []string{`"finish_reason":"tool_calls"`, `"name":"lookup"`, `"total_tokens":14`, `answer`} {
		if !strings.Contains(string(raw), needle) {
			t.Fatalf("lost %s: %s", needle, raw)
		}
	}
}
func TestCrossFormatStreamIsExplicitUntilImplemented(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}'`)
	g.Send = &TranslatorSend{}
	w := httptest.NewRecorder()
	g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/responses", strings.NewReader(`{"model":"gemini/gemini-model","stream":true,"input":"hello"}`)))
	if w.Code != 501 || !strings.Contains(w.Body.String(), "Streaming conversion") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

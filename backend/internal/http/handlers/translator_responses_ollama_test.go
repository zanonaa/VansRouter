package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func responsesConnection() sendConnections {
	return sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream-key"}}}
}

func TestSendResponsesGenericProvider(t *testing.T) {
	var received map[string]any
	sender := &TranslatorSend{
		Connections: responsesConnection(),
		Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.String() != "https://api.perplexity.ai/v1/responses" {
				t.Fatalf("url: %s", r.URL)
			}
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatal(err)
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
						"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":2,\"output_tokens\":1}}}\n\n" +
						"data: [DONE]\n\n")),
			}, nil
		})},
	}
	w := httptest.NewRecorder()
	sender.SendResponses(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), "perplexity-agent", "sonar", map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		"stream":   false,
	}, "openai", false)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if received["input"] == nil || received["stream"] != true {
		t.Fatalf("corpo não convertido para Responses: %v", received)
	}
	var completion map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &completion); err != nil {
		t.Fatal(err)
	}
	choices, _ := completion["choices"].([]any)
	if len(choices) != 1 {
		t.Fatalf("choices: %v", completion)
	}
	message, _ := choices[0].(map[string]any)["message"].(map[string]any)
	if message["content"] != "ok" {
		t.Fatalf("conteúdo: %v", completion)
	}
}

func TestSendResponsesPassesThroughResponsesSource(t *testing.T) {
	var received map[string]any
	sender := &TranslatorSend{
		Connections: responsesConnection(),
		Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatal(err)
			}
			return &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
						"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n" +
						"data: [DONE]\n\n")),
			}, nil
		})},
	}
	w := httptest.NewRecorder()
	sender.SendResponses(w, httptest.NewRequest("POST", "/v1/responses", nil), "perplexity-agent", "sonar", map[string]any{
		"input":  "hello",
		"stream": true,
	}, "openai-responses", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "response.output_text.delta") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if received["input"] != "hello" {
		t.Fatalf("input perdido: %v", received)
	}
}

func TestSendOllamaStreamAndNonStream(t *testing.T) {
	newSender := func(body string) *TranslatorSend {
		return &TranslatorSend{
			Connections: responsesConnection(),
			Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				if !strings.HasSuffix(r.URL.Path, "/api/chat") {
					t.Fatalf("url: %s", r.URL)
				}
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if request["messages"] == nil || request["model"] != "llama" {
					t.Fatalf("request: %v", request)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})},
		}
	}
	streamBody := "{\"model\":\"llama\",\"message\":{\"role\":\"assistant\",\"content\":\"hi\"},\"done\":false}\n" +
		"{\"model\":\"llama\",\"message\":{\"role\":\"assistant\",\"content\":\"!\"},\"done\":true,\"done_reason\":\"stop\",\"prompt_eval_count\":2,\"eval_count\":2}\n"
	w := httptest.NewRecorder()
	newSender(streamBody).SendOllama(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), "ollama-local", "llama", map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		"stream":   true,
	}, "openai", true)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"content":"hi"`) || !strings.Contains(w.Body.String(), `"content":"!"`) || !strings.Contains(w.Body.String(), "[DONE]") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"prompt_tokens":2`) {
		t.Fatalf("usage ausente: %s", w.Body.String())
	}

	nonStreamBody := `{"model":"llama","message":{"role":"assistant","content":"done"},"done":true,"done_reason":"stop","prompt_eval_count":3,"eval_count":4}`
	w = httptest.NewRecorder()
	newSender(nonStreamBody).SendOllama(w, httptest.NewRequest("POST", "/v1/chat/completions", nil), "ollama-local", "llama", map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hello"}},
		"stream":   false,
	}, "openai", false)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"content":"done"`) || !strings.Contains(w.Body.String(), `"total_tokens":7`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestOllamaTargetConvertsToolsAndImages(t *testing.T) {
	body, err := ollamaTarget(map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "text", "text": "look"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,AAAA"}},
			}},
			map[string]any{"role": "assistant", "tool_calls": []any{
				map[string]any{"id": "call_1", "function": map[string]any{"name": "lookup", "arguments": `{"q":"x"}`}},
			}},
			map[string]any{"role": "tool", "tool_call_id": "call_1", "content": "result"},
		},
		"temperature": 0.5,
		"max_tokens":  128,
	}, "llama", true)
	if err != nil {
		t.Fatal(err)
	}
	messages, _ := body["messages"].([]any)
	if len(messages) != 3 {
		t.Fatalf("messages: %v", messages)
	}
	user, _ := messages[0].(map[string]any)
	images, _ := user["images"].([]string)
	if ttsString(user["content"]) != "look" || len(images) != 1 || ttsString(images[0]) != "AAAA" {
		t.Fatalf("user: %v", user)
	}
	assistant, _ := messages[1].(map[string]any)
	calls, _ := assistant["tool_calls"].([]any)
	call, _ := calls[0].(map[string]any)
	function, _ := call["function"].(map[string]any)
	if function["arguments"].(map[string]any)["q"] != "x" {
		t.Fatalf("tool call: %v", assistant)
	}
	tool, _ := messages[2].(map[string]any)
	if tool["tool_name"] != "lookup" || tool["content"] != "result" {
		t.Fatalf("tool: %v", tool)
	}
	options, _ := body["options"].(map[string]any)
	if options["num_predict"] != 128 || options["temperature"] != 0.5 {
		t.Fatalf("options: %v", options)
	}
}

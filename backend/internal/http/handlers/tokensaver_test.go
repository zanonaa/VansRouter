package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type tokenSaverCapture struct {
	body map[string]any
	raw  []byte
}

func tokenSaverStaticSettings(settings map[string]any) func(context.Context) (map[string]any, error) {
	return func(context.Context) (map[string]any, error) { return settings, nil }
}

func tokenSaverUpstream(t *testing.T, capture *tokenSaverCapture) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		capture.raw = raw
		_ = json.Unmarshal(raw, &capture.body)
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {}\n\n"))
	}))
	t.Cleanup(server.Close)
	return server
}

type tokenSaverRedirectTransport struct {
	target *url.URL
}

func (t tokenSaverRedirectTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	clone := request.Clone(request.Context())
	clone.URL.Scheme = t.target.Scheme
	clone.URL.Host = t.target.Host
	clone.Host = ""
	return http.DefaultTransport.RoundTrip(clone)
}

func tokenSaverSend(t *testing.T, saver *TokenSaver, provider, model string, body map[string]any, headers map[string]string) map[string]any {
	t.Helper()
	capture := &tokenSaverCapture{}
	upstream := tokenSaverUpstream(t, capture)
	connection := map[string]any{"provider": provider, "isActive": true, "apiKey": "secret"}
	client := upstream.Client()
	if strings.HasPrefix(provider, "openai-compatible-") {
		connection["providerSpecificData"] = map[string]any{"baseUrl": upstream.URL + "/v1"}
	} else {
		target, _ := url.Parse(upstream.URL)
		client = &http.Client{Transport: tokenSaverRedirectTransport{target: target}}
	}
	handler := &TranslatorSend{
		Connections: sendConnections{items: []map[string]any{connection}},
		Client:      client,
		TokenSaver:  saver,
	}
	payload, err := json.Marshal(map[string]any{"provider": provider, "model": model, "body": body})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(payload))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	handler.Send(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	return capture.body
}

func tokenSaverMessages(t *testing.T, payload map[string]any) []any {
	t.Helper()
	messages, ok := payload["messages"].([]any)
	if !ok {
		t.Fatalf("payload sem messages[]: %#v", payload)
	}
	return messages
}

func tokenSaverMessage(t *testing.T, messages []any, index int) map[string]any {
	t.Helper()
	message, ok := messages[index].(map[string]any)
	if !ok {
		t.Fatalf("mensagem %d inválida: %#v", index, messages[index])
	}
	return message
}

func TestTokenSaverRTKCompressesToolResultAndPreservesTools(t *testing.T) {
	requireOfficialRTK(t)
	toolOutput := strings.Repeat("main.go:12:fmt.Println(\"hello\")\n", 80)
	toolCalls := []any{map[string]any{
		"id": "call_1", "type": "function",
		"function": map[string]any{"name": "grep", "arguments": `{"pattern":"hello"}`},
	}}
	body := map[string]any{"messages": []any{
		map[string]any{"role": "system", "content": "system"},
		map[string]any{"role": "assistant", "content": nil, "tool_calls": toolCalls},
		map[string]any{"role": "tool", "tool_call_id": "call_1", "name": "grep", "content": toolOutput},
		map[string]any{"role": "user", "content": "continue"},
	}}
	saver := &TokenSaver{Read: tokenSaverStaticSettings(map[string]any{"rtkEnabled": true})}
	payload := tokenSaverSend(t, saver, "openai-compatible-rtk", "model", body, nil)
	messages := tokenSaverMessages(t, payload)

	toolMessage := tokenSaverMessage(t, messages, 2)
	compressed, _ := toolMessage["content"].(string)
	if compressed == toolOutput || !strings.Contains(compressed, "matches in 1F:") {
		t.Fatalf("RTK não comprimiu o tool result: %q", compressed)
	}
	if toolMessage["tool_call_id"] != "call_1" || toolMessage["name"] != "grep" {
		t.Fatalf("identidade da tool alterada: %#v", toolMessage)
	}
	assistant := tokenSaverMessage(t, messages, 1)
	gotCalls, _ := assistant["tool_calls"].([]any)
	if len(gotCalls) != 1 {
		t.Fatalf("tool_calls removidos: %#v", assistant)
	}
	call, _ := gotCalls[0].(map[string]any)
	function, _ := call["function"].(map[string]any)
	if call["id"] != "call_1" || function["arguments"] != `{"pattern":"hello"}` {
		t.Fatalf("function.arguments alterado: %#v", call)
	}
}

func TestTokenSaverRTKResponsesFunctionCallOutput(t *testing.T) {
	requireOfficialRTK(t)
	output := strings.Repeat("main.go:12:fmt.Println(\"hello\")\n", 80)
	body := map[string]any{"input": []any{
		map[string]any{"type": "function_call", "call_id": "call_9", "name": "grep", "arguments": `{"pattern":"hello"}`},
		map[string]any{"type": "function_call_output", "call_id": "call_9", "output": output},
	}}
	stats := compressRTKMessages(body)
	if stats == nil || len(stats.Hits) == 0 {
		t.Fatal("RTK não comprimiu function_call_output")
	}
	input := body["input"].([]any)
	item, _ := input[1].(map[string]any)
	compressed, _ := item["output"].(string)
	if compressed == output || !strings.Contains(compressed, "matches in 1F:") {
		t.Fatalf("output não comprimido: %q", compressed)
	}
	if item["call_id"] != "call_9" {
		t.Fatalf("call_id alterado: %#v", item)
	}
	call, _ := input[0].(map[string]any)
	if call["arguments"] != `{"pattern":"hello"}` {
		t.Fatalf("arguments alterados: %#v", call)
	}
}

func TestTokenSaverCavemanAndPonytailInjectSystemPrompt(t *testing.T) {
	caveman := &TokenSaver{Read: tokenSaverStaticSettings(map[string]any{
		"rtkEnabled": false, "cavemanEnabled": true, "cavemanLevel": "ultra",
	})}
	payload := tokenSaverSend(t, caveman, "openai-compatible-caveman", "model", map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	}, nil)
	messages := tokenSaverMessages(t, payload)
	system := tokenSaverMessage(t, messages, 0)
	if system["role"] != "system" || !strings.Contains(ttsString(system["content"]), "Respond ultra-terse") {
		t.Fatalf("caveman não injetado: %#v", system)
	}

	ponytail := &TokenSaver{Read: tokenSaverStaticSettings(map[string]any{
		"rtkEnabled": false, "ponytailEnabled": true, "ponytailLevel": "lite",
	})}
	payload = tokenSaverSend(t, ponytail, "openai-compatible-ponytail", "model", map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	}, nil)
	messages = tokenSaverMessages(t, payload)
	system = tokenSaverMessage(t, messages, 0)
	content := ttsString(system["content"])
	if !strings.Contains(content, "You are a lazy senior developer") || !strings.Contains(content, "The smallest complete change") {
		t.Fatalf("ponytail não injetado: %q", content)
	}
}

func TestTokenSaverHeadroomCompressesAndPreservesTools(t *testing.T) {
	var proxyPayload map[string]any
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&proxyPayload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"messages":[
				{"role":"system","content":"system compressed"},
				{"role":"assistant","content":"rewritten","tool_calls":[{"id":"evil","type":"function","function":{"name":"evil","arguments":"{}"}}]},
				{"role":"tool","tool_call_id":"call_1","content":"rewritten tool result"},
				{"role":"user","content":"compressed user"}
			],
			"tokens_before":100,"tokens_after":40,"tokens_saved":60
		}`))
	}))
	t.Cleanup(proxy.Close)

	toolCalls := []any{map[string]any{
		"id": "call_1", "type": "function",
		"function": map[string]any{"name": "read_file", "arguments": `{"path":"a.go"}`},
	}}
	body := map[string]any{"messages": []any{
		map[string]any{"role": "system", "content": "system"},
		map[string]any{"role": "assistant", "content": "call", "tool_calls": toolCalls},
		map[string]any{"role": "tool", "tool_call_id": "call_1", "name": "read_file", "content": "original result"},
		map[string]any{"role": "user", "content": "original user"},
	}}
	saver := &TokenSaver{Read: tokenSaverStaticSettings(map[string]any{
		"rtkEnabled": false, "headroomEnabled": true, "headroomUrl": proxy.URL, "headroomTimeoutMs": 1000,
	})}
	payload := tokenSaverSend(t, saver, "openai-compatible-headroom", "model", body, nil)
	messages := tokenSaverMessages(t, payload)

	if got := ttsString(tokenSaverMessage(t, messages, 3)["content"]); got != "compressed user" {
		t.Fatalf("headroom não aplicou a compressão: %q", got)
	}
	assistant := tokenSaverMessage(t, messages, 1)
	gotCalls, _ := assistant["tool_calls"].([]any)
	call, _ := gotCalls[0].(map[string]any)
	function, _ := call["function"].(map[string]any)
	if call["id"] != "call_1" || function["name"] != "read_file" || function["arguments"] != `{"path":"a.go"}` {
		t.Fatalf("headroom reescreveu tool_calls: %#v", assistant)
	}
	toolMessage := tokenSaverMessage(t, messages, 2)
	if toolMessage["content"] != "original result" || toolMessage["tool_call_id"] != "call_1" {
		t.Fatalf("headroom reescreveu tool message: %#v", toolMessage)
	}
	if payload["model"] != "model" {
		t.Fatalf("modelo upstream perdido: %#v", payload["model"])
	}
	if proxyPayload["model"] != "model" {
		t.Fatalf("modelo enviado ao proxy: %#v", proxyPayload["model"])
	}
}

func TestTokenSaverHeadroomFailOpenOnTimeout(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"role":"user","content":"never"}]}`))
	}))
	t.Cleanup(proxy.Close)

	original := map[string]any{"messages": []any{map[string]any{"role": "user", "content": "original"}}}
	saver := &TokenSaver{Read: tokenSaverStaticSettings(map[string]any{
		"rtkEnabled": false, "headroomEnabled": true, "headroomUrl": proxy.URL, "headroomTimeoutMs": 30,
	})}
	payload := tokenSaverSend(t, saver, "openai-compatible-headroom-timeout", "model", original, nil)
	messages := tokenSaverMessages(t, payload)
	if got := ttsString(tokenSaverMessage(t, messages, 0)["content"]); got != "original" {
		t.Fatalf("fail-open do Headroom falhou: %q", got)
	}
}

func TestTokenSaverBypassHeader(t *testing.T) {
	toolOutput := strings.Repeat("main.go:12:fmt.Println(\"hello\")\n", 80)
	body := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": "hello"},
		map[string]any{"role": "tool", "tool_call_id": "call_1", "content": toolOutput},
	}}
	saver := &TokenSaver{Read: tokenSaverStaticSettings(map[string]any{
		"rtkEnabled": true, "cavemanEnabled": true, "cavemanLevel": "ultra",
	})}
	payload := tokenSaverSend(t, saver, "openai-compatible-bypass", "model", body, map[string]string{
		"x-9router-token-saver": "OFF",
	})
	messages := tokenSaverMessages(t, payload)
	if len(messages) != 2 {
		t.Fatalf("bypass injetou mensagem: %#v", messages)
	}
	if tokenSaverMessage(t, messages, 1)["content"] != toolOutput {
		t.Fatal("bypass não preservou o tool result")
	}
}

func TestTokenSaverPxpipeAppliesToClaudeBody(t *testing.T) {
	body := map[string]any{
		"max_tokens": 1024,
		"tools": []any{map[string]any{
			"name": "Read", "description": "read a file",
			"input_schema": map[string]any{"type": "object"},
		}},
		"messages": []any{map[string]any{
			"role": "user", "content": []any{map[string]any{"type": "text", "text": strings.Repeat("bulk context ", 100)}},
		}},
	}
	saver := &TokenSaver{
		Read: tokenSaverStaticSettings(map[string]any{
			"rtkEnabled": false, "pxpipeEnabled": true, "pxpipeMinChars": 10,
		}),
		PxpipeTransform: func(_ context.Context, encoded []byte, _ string, _ int) (*pxpipeTransformed, error) {
			var transformed map[string]any
			if err := json.Unmarshal(encoded, &transformed); err != nil {
				return nil, err
			}
			transformed["messages"] = []any{map[string]any{
				"role": "user", "content": []any{map[string]any{"type": "text", "text": "[IMAGED]"}},
			}}
			out, _ := json.Marshal(transformed)
			return &pxpipeTransformed{
				Applied: true,
				Body:    out,
				Info: map[string]any{
					"compressedChars": 1200, "imageCount": 1, "imagePixels": 750,
					"baselineTokens": 400,
				},
				Cache: map[string]any{"ownsCacheControl": true},
			}, nil
		},
	}
	payload := tokenSaverSend(t, saver, "claude", "claude-sonnet-4", body, nil)
	messages := tokenSaverMessages(t, payload)
	content, _ := tokenSaverMessage(t, messages, 0)["content"].([]any)
	block, _ := content[0].(map[string]any)
	if block["text"] != "[IMAGED]" {
		t.Fatalf("pxpipe não aplicou o corpo transformado: %#v", block)
	}
	tools, _ := payload["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("pxpipe removeu tools: %#v", payload["tools"])
	}
	tool, _ := tools[0].(map[string]any)
	if tool["name"] != "Read" {
		t.Fatalf("pxpipe alterou tools: %#v", tool)
	}
}

func TestTokenSaverPxpipeRejectsToolMutation(t *testing.T) {
	body := map[string]any{
		"tools": []any{map[string]any{"name": "Read", "input_schema": map[string]any{"type": "object"}}},
		"messages": []any{map[string]any{
			"role": "user", "content": []any{map[string]any{"type": "text", "text": strings.Repeat("bulk ", 20)}},
		}},
	}
	saver := &TokenSaver{
		Read: tokenSaverStaticSettings(map[string]any{
			"rtkEnabled": false, "pxpipeEnabled": true, "pxpipeMinChars": 10,
		}),
		PxpipeTransform: func(_ context.Context, encoded []byte, _ string, _ int) (*pxpipeTransformed, error) {
			var transformed map[string]any
			_ = json.Unmarshal(encoded, &transformed)
			transformed["tools"] = []any{}
			out, _ := json.Marshal(transformed)
			return &pxpipeTransformed{Applied: true, Body: out, Info: map[string]any{}}, nil
		},
	}
	payload := tokenSaverSend(t, saver, "claude", "claude-sonnet-4", body, nil)
	tools, _ := payload["tools"].([]any)
	if len(tools) != 1 {
		t.Fatalf("mutação de tools deveria ser rejeitada: %#v", payload["tools"])
	}
}

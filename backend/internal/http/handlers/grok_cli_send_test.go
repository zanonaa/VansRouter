package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func TestOpenAIResponsesTargetTranslatesChatBody(t *testing.T) {
	source := map[string]any{
		"messages": []any{
			map[string]any{"role": "system", "content": "be nice"},
			map[string]any{"role": "user", "content": []any{
				map[string]any{"type": "text", "text": "hi"},
				map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://x/img.png", "detail": "high"}},
			}},
			map[string]any{
				"role": "assistant", "content": "", "reasoning_content": "thinking", "encrypted_content": "cipher",
				"tool_calls": []any{map[string]any{"id": "call_1", "function": map[string]any{"name": "lookup", "arguments": `{"q":1}`}}},
			},
			map[string]any{"role": "tool", "tool_call_id": "call_1", "content": "result"},
		},
		"tools": []any{map[string]any{"type": "function", "function": map[string]any{
			"name": "lookup", "parameters": map[string]any{"type": "object"},
		}}},
		"max_tokens":       100,
		"temperature":      0.2,
		"reasoning_effort": "medium",
	}
	out, err := openAIResponsesTarget(source, "grok-4.7")
	if err != nil {
		t.Fatal(err)
	}
	if out["instructions"] != "be nice" || out["stream"] != true || out["store"] != false {
		t.Fatalf("header fields: %#v", out)
	}
	if out["max_output_tokens"] != 100 {
		t.Fatalf("max_output_tokens: %#v", out["max_output_tokens"])
	}
	reasoning, _ := out["reasoning"].(map[string]any)
	if reasoning["effort"] != "medium" || reasoning["summary"] != "auto" {
		t.Fatalf("reasoning: %#v", reasoning)
	}
	input, _ := out["input"].([]any)
	seen := map[string]int{}
	for _, raw := range input {
		item, _ := raw.(map[string]any)
		seen[ttsString(item["type"])]++
	}
	for _, kind := range []string{"message", "reasoning", "function_call", "function_call_output"} {
		if seen[kind] == 0 {
			t.Fatalf("missing %s in %#v", kind, input)
		}
	}
	developer, _ := input[0].(map[string]any)
	if developer["role"] != "developer" {
		t.Fatalf("system not preserved in input: %#v", developer)
	}
	reasoningItem, _ := input[2].(map[string]any)
	if reasoningItem["type"] != "reasoning" || reasoningItem["encrypted_content"] != "cipher" {
		t.Fatalf("reasoning continuity lost: %#v", reasoningItem)
	}
	user, _ := input[1].(map[string]any)
	blocks, _ := user["content"].([]any)
	image, _ := blocks[1].(map[string]any)
	if image["type"] != "input_image" || image["detail"] != "high" {
		t.Fatalf("image block: %#v", image)
	}
	tools, _ := out["tools"].([]any)
	tool, _ := tools[0].(map[string]any)
	if tool["type"] != "function" || tool["name"] != "lookup" || tool["function"] != nil {
		t.Fatalf("tool not flattened: %#v", tool)
	}
}

func TestOpenAIResponsesTargetPassesExistingInput(t *testing.T) {
	out, err := openAIResponsesTarget(map[string]any{
		"input":      []any{map[string]any{"type": "message", "role": "user", "content": "hi"}},
		"max_tokens": 50, "stream": false,
	}, "grok-4.7")
	if err != nil {
		t.Fatal(err)
	}
	if out["stream"] != true || out["max_output_tokens"] != 50 {
		t.Fatalf("passthrough: %#v", out)
	}
	if _, exists := out["max_tokens"]; exists {
		t.Fatal("max_tokens should be removed")
	}
}

func TestResponsesStreamConvertsToOpenAI(t *testing.T) {
	w := httptest.NewRecorder()
	s := &streamConversion{destination: w, source: "openai", target: "openai-responses", model: "grok-4.7", textBlock: -1, serverTool: -1}
	frames := []string{
		`{"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		`{"type":"response.reasoning_summary_text.delta","delta":"think "}`,
		`{"type":"response.output_text.delta","delta":"olá"}`,
		`{"type":"response.output_item.added","item":{"id":"fc_1","type":"function_call","call_id":"call_1","name":"lookup"}}`,
		`{"type":"response.function_call_arguments.delta","item_id":"fc_1","delta":"{\"q\":1}"}`,
		`{"type":"response.output_item.done","item":{"id":"fc_1","type":"function_call"}}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","usage":{"input_tokens":10,"output_tokens":5,"input_tokens_details":{"cached_tokens":2}}}}`,
	}
	for _, frame := range frames {
		if _, err := s.Write([]byte("data: " + frame + "\n\n")); err != nil {
			t.Fatal(err)
		}
	}
	s.Write([]byte("data: [DONE]\n\n"))
	s.finish()
	body := w.Body.String()
	for _, needle := range []string{
		`"reasoning_content":"think "`, `"content":"olá"`, `"id":"call_1"`, `"name":"lookup"`,
		`"arguments":"{\"q\":1}"`, `"finish_reason":"tool_calls"`, `"prompt_tokens":10`,
		`"completion_tokens":5`, `"cached_tokens":2`, "data: [DONE]",
	} {
		if !strings.Contains(body, needle) {
			t.Fatalf("lost %s: %s", needle, body)
		}
	}
	if strings.Count(body, "[DONE]") != 1 {
		t.Fatalf("duplicate completion sentinel: %s", body)
	}
}

func TestResponsesNonStreamConvertsToOpenAI(t *testing.T) {
	native := map[string]any{
		"id": "resp_1", "status": "completed",
		"output": []any{
			map[string]any{"type": "reasoning", "summary": []any{map[string]any{"type": "summary_text", "text": "thinking"}}},
			map[string]any{"type": "message", "content": []any{map[string]any{"type": "output_text", "text": "olá"}}},
			map[string]any{"type": "function_call", "call_id": "call_1", "name": "lookup", "arguments": `{"q":1}`},
		},
		"usage": map[string]any{"input_tokens": 10, "output_tokens": 5},
	}
	out, err := convertChatResponse(native, "openai-responses", "openai", "grok-4.7")
	if err != nil {
		t.Fatal(err)
	}
	choice, _ := out["choices"].([]any)
	first, _ := choice[0].(map[string]any)
	message, _ := first["message"].(map[string]any)
	if message["content"] != "olá" || message["reasoning_content"] != "thinking" || first["finish_reason"] != "tool_calls" {
		t.Fatalf("converted: %#v", out)
	}
	calls, _ := message["tool_calls"].([]any)
	call, _ := calls[0].(map[string]any)
	if call["id"] != "call_1" {
		t.Fatalf("tool call: %#v", call)
	}
}

func TestPrepareGrokCLIBodyNormalizesInputAndTools(t *testing.T) {
	body := map[string]any{
		"model": "grok-4.5-high",
		"input": []any{
			map[string]any{"type": "message", "role": "system", "content": "You are Grok"},
			map[string]any{"type": "message", "role": "user", "content": "hi", "id": "msg_server_id"},
			map[string]any{"type": "item_reference", "id": "rs_abc"},
			"rs_should_drop",
			map[string]any{"type": "custom_tool_call", "call_id": "call-custom", "name": "exec", "input": "run this"},
			map[string]any{"type": "custom_tool_call_output", "call_id": "call-custom", "output": []any{map[string]any{"type": "input_text", "text": "done"}}},
			map[string]any{"type": "function_call_output", "call_id": "orphan", "output": "no call"},
		},
		"tools": []any{
			map[string]any{"type": "function", "function": map[string]any{"name": "run", "description": "Run", "parameters": map[string]any{"type": "object", "properties": map[string]any{"cmd": map[string]any{"type": "string"}}}}},
			map[string]any{"type": "web_search"},
			map[string]any{"type": "custom", "name": "exec"},
		},
		"tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "run"}},
		"stream":      false,
		"max_tokens":  100,
		"user":        "cursor-user",
		"temperature": 0.7,
	}
	out, model, err := prepareGrokCLIBody(body, "grok-4.5-high")
	if err != nil {
		t.Fatal(err)
	}
	if model != "grok-4.5" || out["model"] != "grok-4.5" {
		t.Fatalf("model: %s %#v", model, out["model"])
	}
	if out["stream"] != true || out["store"] != false {
		t.Fatalf("stream/store: %#v", out)
	}
	reasoning, _ := out["reasoning"].(map[string]any)
	if reasoning["effort"] != "high" || reasoning["summary"] != "concise" {
		t.Fatalf("reasoning: %#v", reasoning)
	}
	include, _ := out["include"].([]any)
	if len(include) != 1 || include[0] != "reasoning.encrypted_content" {
		t.Fatalf("include: %#v", include)
	}
	input, _ := out["input"].([]any)
	if len(input) != 4 {
		t.Fatalf("input normalization: %#v", input)
	}
	system, _ := input[0].(map[string]any)
	if system["role"] != "system" {
		t.Fatalf("grok keeps system (HAR parity): %#v", system)
	}
	if input[1].(map[string]any)["id"] != nil {
		t.Fatalf("server id kept: %#v", input[1])
	}
	call, _ := input[2].(map[string]any)
	if call["type"] != "function_call" || call["arguments"] != `{"input":"run this"}` {
		t.Fatalf("custom call: %#v", call)
	}
	tools, _ := out["tools"].([]any)
	if len(tools) != 3 {
		t.Fatalf("tools: %#v", tools)
	}
	function, _ := tools[0].(map[string]any)
	if function["name"] != "run" || function["function"] != nil {
		t.Fatalf("function tool: %#v", function)
	}
	hosted, _ := tools[1].(map[string]any)
	if hosted["type"] != "web_search" {
		t.Fatalf("hosted tool: %#v", hosted)
	}
	custom, _ := tools[2].(map[string]any)
	parameters, _ := custom["parameters"].(map[string]any)
	if parameters["required"] == nil {
		t.Fatalf("custom tool parameters: %#v", custom)
	}
	choice, _ := out["tool_choice"].(map[string]any)
	if choice["type"] != "function" || choice["name"] != "run" {
		t.Fatalf("tool_choice: %#v", choice)
	}
	for _, key := range []string{"messages", "max_tokens", "user"} {
		if _, exists := out[key]; exists {
			t.Fatalf("%s should be dropped", key)
		}
	}
}

func TestPrepareGrokCLIBodyOmitsEffortForGrokBuild(t *testing.T) {
	out, _, err := prepareGrokCLIBody(map[string]any{
		"model": "grok-build", "input": "hi", "reasoning": map[string]any{"effort": "max"},
	}, "grok-build")
	if err != nil {
		t.Fatal(err)
	}
	reasoning, _ := out["reasoning"].(map[string]any)
	if reasoning["effort"] != nil || reasoning["summary"] != "concise" {
		t.Fatalf("effort should be omitted: %#v", reasoning)
	}
	include, _ := out["include"].([]any)
	if len(include) != 1 {
		t.Fatalf("include missing: %#v", out)
	}
}

func TestFoldOpenAIStreamBuildsCompletion(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"id":"chatcmpl-1","model":"grok-4.7","choices":[{"index":0,"delta":{"reasoning_content":"think "},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","model":"grok-4.7","choices":[{"index":0,"delta":{"content":"olá"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","model":"grok-4.7","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":1}"}}]},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl-1","model":"grok-4.7","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`,
		`data: [DONE]`,
	}, "\n")
	out, err := foldOpenAIStream([]byte(raw), "grok-4.7")
	if err != nil {
		t.Fatal(err)
	}
	choices, _ := out["choices"].([]any)
	choice, _ := choices[0].(map[string]any)
	message, _ := choice["message"].(map[string]any)
	if message["content"] != "olá" || message["reasoning_content"] != "think " {
		t.Fatalf("message: %#v", message)
	}
	calls, _ := message["tool_calls"].([]any)
	call, _ := calls[0].(map[string]any)
	function, _ := call["function"].(map[string]any)
	if call["id"] != "call_1" || function["name"] != "lookup" || function["arguments"] != `{"q":1}` {
		t.Fatalf("tool call: %#v", call)
	}
	if choice["finish_reason"] != "tool_calls" {
		t.Fatalf("finish: %#v", choice)
	}
	usage, _ := out["usage"].(map[string]any)
	if usage["total_tokens"] != float64(15) {
		t.Fatalf("usage: %#v", usage)
	}
}

func TestSendGrokCLIStreamsAndFolds(t *testing.T) {
	upstream := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"status\":\"in_progress\"}}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"olá\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"status\":\"completed\",\"usage\":{\"input_tokens\":12,\"output_tokens\":3}}}\n\n" +
		"data: [DONE]\n\n"
	for _, stream := range []bool{true, false} {
		t.Run(map[bool]string{true: "stream", false: "json"}[stream], func(t *testing.T) {
			grokTurns.Lock()
			grokTurns.entries = map[string]grokTurnEntry{}
			grokTurns.Unlock()
			db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Close() })
			_, err = db.Exec(`CREATE TABLE apiKeys(id TEXT,key TEXT,isActive INTEGER); INSERT INTO apiKeys VALUES('k','secret',1);
				CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT);
				INSERT INTO providerConnections VALUES('conn-grok','grok-cli','oauth','Grok',NULL,1,1,'{"accessToken":"tok","providerSpecificData":{"email":"u@example.com","userId":"uid-1"}}','now','now');`)
			if err != nil {
				t.Fatal(err)
			}
			requests := 0
			send := &TranslatorSend{Connections: database.Connections{DB: db}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.URL.String() != "https://cli-chat-proxy.grok.com/v1/responses" {
					t.Fatalf("url: %s", r.URL)
				}
				for header, want := range map[string]string{
					"Authorization": "Bearer tok", "X-Grok-Session-Id": "conn-grok", "X-Grok-Conv-Id": "conn-grok",
					"X-Grok-Turn-Idx": "1", "X-Grok-Model-Override": "grok-4.7", "X-Email": "u@example.com", "X-Userid": "uid-1",
				} {
					if got := r.Header.Get(header); got != want {
						t.Fatalf("%s = %q, want %q", header, got, want)
					}
				}
				if r.Header.Get("X-Grok-Req-Id") == "" || !strings.Contains(r.Header.Get("User-Agent"), "grok-shell") {
					t.Fatalf("missing CLI fingerprint: %#v", r.Header)
				}
				var body map[string]any
				encoded, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(encoded, &body); err != nil {
					t.Fatal(err)
				}
				if body["model"] != "grok-4.7" || body["stream"] != true || body["store"] != false {
					t.Fatalf("body: %s", encoded)
				}
				if body["messages"] != nil || body["max_tokens"] != nil {
					t.Fatalf("chat leftovers: %s", encoded)
				}
				reasoning, _ := body["reasoning"].(map[string]any)
				if reasoning["summary"] != "concise" {
					t.Fatalf("reasoning: %s", encoded)
				}
				input, _ := body["input"].([]any)
				if len(input) == 0 || input[0].(map[string]any)["role"] != "user" {
					t.Fatalf("input: %s", encoded)
				}
				return &http.Response{
					StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
					Body: io.NopCloser(strings.NewReader(upstream)),
				}, nil
			})}}
			gateway := &Gateway{DB: db, Send: send}
			payload := `{"model":"gcli/grok-4.7","messages":[{"role":"user","content":"hi"}],"stream":` + map[bool]string{true: "true", false: "false"}[stream] + `}`
			request := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(payload))
			request.Header.Set("Authorization", "Bearer secret")
			w := httptest.NewRecorder()
			gateway.OpenAIChat(w, request)
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if requests != 1 {
				t.Fatalf("upstream calls: %d", requests)
			}
			if stream {
				if !strings.Contains(w.Body.String(), `"content":"olá"`) || !strings.Contains(w.Body.String(), "data: [DONE]") {
					t.Fatalf("stream body: %s", w.Body.String())
				}
				return
			}
			var completion map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &completion); err != nil {
				t.Fatalf("json: %v %s", err, w.Body.String())
			}
			if completion["object"] != "chat.completion" {
				t.Fatalf("completion: %s", w.Body.String())
			}
			choices, _ := completion["choices"].([]any)
			first, _ := choices[0].(map[string]any)
			message, _ := first["message"].(map[string]any)
			if message["content"] != "olá" {
				t.Fatalf("message: %s", w.Body.String())
			}
		})
	}
}

func TestSendGrokCLIPropagatesQuotaError(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE apiKeys(id TEXT,key TEXT,isActive INTEGER); INSERT INTO apiKeys VALUES('k','secret',1);
		CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT);
		INSERT INTO providerConnections VALUES('conn-quota','grok-cli','oauth','Grok',NULL,1,1,'{"accessToken":"tok"}','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	send := &TranslatorSend{Connections: database.Connections{DB: db}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 402, Header: http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(`{"code":"personal-team-blocked","error":"Grok Build usage balance exhausted"}`)),
		}, nil
	})}}
	gateway := &Gateway{DB: db, Send: send}
	request := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"gcli/grok-4.7","messages":[{"role":"user","content":"hi"}]}`))
	request.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	gateway.OpenAIChat(w, request)
	if w.Code != 402 || !strings.Contains(w.Body.String(), "usage balance exhausted") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestGrokTurnIndexIsMonotonicPerSession(t *testing.T) {
	grokTurns.Lock()
	grokTurns.entries = map[string]grokTurnEntry{}
	grokTurns.Unlock()
	user := map[string]any{"type": "message", "role": "user", "content": "hi"}
	if got := grokResolveTurnIdx("s1", []any{user}); got != 1 {
		t.Fatalf("first turn: %d", got)
	}
	if got := grokResolveTurnIdx("s1", []any{user}); got != 2 {
		t.Fatalf("delta turn should advance: %d", got)
	}
	full := []any{user, map[string]any{"type": "message", "role": "assistant", "content": "ok"}, user, user}
	if got := grokResolveTurnIdx("s1", full); got != 3 {
		t.Fatalf("full history turn: %d", got)
	}
	if got := grokResolveTurnIdx("s2", []any{user}); got != 1 {
		t.Fatalf("session isolation: %d", got)
	}
}

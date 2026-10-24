package handlers

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func codexTestConnection(t *testing.T) (*sql.DB, string) {
	t.Helper()
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	_, err = db.Exec(`CREATE TABLE apiKeys(id TEXT,key TEXT,isActive INTEGER); INSERT INTO apiKeys VALUES('k','secret',1);
		CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT);
		INSERT INTO providerConnections VALUES('conn-codex','codex','oauth','Codex',NULL,1,1,'{"accessToken":"tok","providerSpecificData":{"workspaceId":"ws-1","chatgptAccountId":"acct-1"}}','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	return db, "secret"
}

func TestPrepareCodexBodyFiltersToolsAndInjectsDefaults(t *testing.T) {
	body := map[string]any{
		"model": "gpt-5.6-sol",
		"input": []any{
			map[string]any{"type": "message", "role": "system", "content": "be nice"},
			map[string]any{"type": "message", "role": "user", "content": "hi", "id": "msg_server"},
			map[string]any{"type": "item_reference", "id": "rs_ref"},
			"rs_dangling",
		},
		"tools": []any{
			map[string]any{"type": "function", "function": map[string]any{
				"name": "lookup", "description": "Find",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"code": map[string]any{"type": "string", "pattern": `\p{L}+`},
						"safe": map[string]any{"type": "string", "pattern": `^[a-z]+$`},
					},
				},
			}},
			map[string]any{"type": "web_search"},
			map[string]any{"type": "made_up"},
			map[string]any{"type": "custom", "name": "exec"},
			map[string]any{"type": "function", "function": map[string]any{"name": "gone"}},
			map[string]any{"type": "namespace", "name": "ns", "tools": []any{
				map[string]any{"name": "nested", "parameters": map[string]any{"pattern": `\P{ASCII}`}},
			}},
		},
		"tool_choice":       map[string]any{"type": "function", "name": "missing"},
		"max_output_tokens": 123,
		"temperature":       0.5,
		"metadata":          map[string]any{"k": "v"},
		"service_tier":      "fast",
	}
	headers := http.Header{"X-Session-Id": []string{"client-session"}}
	out, model, session, err := prepareCodexBody(body, "gpt-5.6-sol", map[string]any{"id": "conn-codex"}, headers)
	if err != nil {
		t.Fatal(err)
	}
	if model != "gpt-5.6-sol" || out["model"] != "gpt-5.6-sol" {
		t.Fatalf("model: %s %#v", model, out["model"])
	}
	if session != "client-session" || out["prompt_cache_key"] != "client-session" {
		t.Fatalf("session: %s %#v", session, out["prompt_cache_key"])
	}
	if out["stream"] != true || out["store"] != false || out["service_tier"] != "priority" {
		t.Fatalf("fixed fields: %#v", out)
	}
	if !strings.Contains(ttsString(out["instructions"]), "You are Codex") {
		t.Fatal("default instructions not injected")
	}
	for _, key := range []string{"max_output_tokens", "temperature", "metadata"} {
		if _, exists := out[key]; exists {
			t.Fatalf("%s should be stripped", key)
		}
	}
	input, _ := out["input"].([]any)
	if len(input) != 2 {
		t.Fatalf("input: %#v", input)
	}
	system, _ := input[0].(map[string]any)
	if system["role"] != "developer" {
		t.Fatalf("system role: %#v", system)
	}
	user, _ := input[1].(map[string]any)
	if user["id"] != nil {
		t.Fatalf("server id kept: %#v", user)
	}
	tools, _ := out["tools"].([]any)
	seen := map[string]map[string]any{}
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		seen[ttsString(tool["type"])+":"+codexToolName(tool)] = tool
	}
	if seen["function:lookup"] == nil || seen["function:gone"] == nil || seen["web_search:"] == nil || seen["custom:exec"] == nil || seen["namespace:ns"] == nil {
		t.Fatalf("tools filtered wrong: %#v", tools)
	}
	if len(tools) != 5 {
		t.Fatalf("unexpected tools: %#v", tools)
	}
	lookup := seen["function:lookup"]
	parameters, _ := lookup["parameters"].(map[string]any)
	properties, _ := parameters["properties"].(map[string]any)
	if _, exists := properties["code"].(map[string]any)["pattern"]; exists {
		t.Fatalf("unicode pattern kept: %#v", properties["code"])
	}
	if properties["safe"].(map[string]any)["pattern"] != `^[a-z]+$` {
		t.Fatalf("safe pattern removed: %#v", properties["safe"])
	}
	if _, exists := out["tool_choice"]; exists {
		t.Fatalf("tool_choice should be dropped: %#v", out["tool_choice"])
	}
	reasoning, _ := out["reasoning"].(map[string]any)
	if reasoning["effort"] != "low" || reasoning["summary"] != "auto" {
		t.Fatalf("reasoning: %#v", reasoning)
	}
	include, _ := out["include"].([]any)
	if len(include) != 1 || include[0] != "reasoning.encrypted_content" {
		t.Fatalf("include: %#v", include)
	}
	if _, exists := out["parallel_tool_calls"]; exists {
		t.Fatal("parallel_tool_calls should be removed for non-lite")
	}
}

func TestPrepareCodexBodyLitePrefix(t *testing.T) {
	out, model, _, err := prepareCodexBody(map[string]any{
		"model": "gpt-6.1-sol",
		"input": []any{map[string]any{"type": "message", "role": "user", "content": "hi"}},
		"tools": []any{map[string]any{"type": "function", "name": "lookup", "parameters": map[string]any{"type": "object"}}},
	}, "gpt-6.1-sol", map[string]any{"id": "conn-codex"}, http.Header{"X-Session-Id": []string{"s1"}})
	if err != nil {
		t.Fatal(err)
	}
	if model != "gpt-6.1-sol" || out["instructions"] != "" || out["tools"] != nil {
		t.Fatalf("lite shape: %#v", out)
	}
	if out["parallel_tool_calls"] != false || out["tool_choice"] != "auto" {
		t.Fatalf("lite flags: %#v", out)
	}
	input, _ := out["input"].([]any)
	if len(input) != 3 {
		t.Fatalf("lite input: %#v", input)
	}
	prefix, _ := input[0].(map[string]any)
	if prefix["type"] != "additional_tools" || prefix["role"] != "developer" {
		t.Fatalf("additional_tools prefix: %#v", prefix)
	}
	prefixTools, _ := prefix["tools"].([]any)
	if len(prefixTools) != 1 || ttsString(prefixTools[0].(map[string]any)["name"]) != "lookup" {
		t.Fatalf("prefix tools: %#v", prefixTools)
	}
	instructions, _ := input[1].(map[string]any)
	if instructions["role"] != "developer" || instructions["type"] != "message" {
		t.Fatalf("instructions item: %#v", instructions)
	}
	reasoning, _ := out["reasoning"].(map[string]any)
	if reasoning["effort"] != "medium" || reasoning["context"] != "all_turns" {
		t.Fatalf("lite reasoning: %#v", reasoning)
	}
	if _, exists := reasoning["summary"]; exists {
		t.Fatalf("lite reasoning must not set summary: %#v", reasoning)
	}
}

func TestPrepareCodexBodyLiteFallsBackToResponsesForWebSearch(t *testing.T) {
	out, _, _, err := prepareCodexBody(map[string]any{
		"model": "gpt-6.1-sol",
		"input": []any{
			map[string]any{"type": "additional_tools", "role": "developer", "tools": []any{map[string]any{"type": "web_search"}}},
			map[string]any{"type": "message", "role": "user", "content": "hi"},
		},
		"tools": []any{map[string]any{"type": "web_search"}},
	}, "gpt-6.1-sol", map[string]any{"id": "conn-codex"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if out["instructions"] == "" || out["tools"] == nil {
		t.Fatalf("web_search must use full Responses: %#v", out)
	}
	input, _ := out["input"].([]any)
	for _, raw := range input {
		if ttsString(raw.(map[string]any)["type"]) == "additional_tools" {
			t.Fatalf("additional_tools should be moved out: %#v", input)
		}
	}
	reasoning, _ := out["reasoning"].(map[string]any)
	if reasoning["effort"] != "low" || reasoning["summary"] != "auto" {
		t.Fatalf("non-lite reasoning: %#v", reasoning)
	}
}

func TestCodexThinkingLevels(t *testing.T) {
	cases := map[string]string{
		"gpt-6.1-sol|none":    "low",
		"gpt-6.1-sol|max":     "max",
		"gpt-6.1-sol|minimal": "low",
		"gpt-5.6-sol|ultra":   "ultra",
		"gpt-5.6-terra|ultra": "ultra",
		"gpt-5.6-luna|ultra":  "max",
		"gpt-6-astra|max":     "max",
		"gpt-5.3-codex|high":  "high",
		"gpt-5.3-codex|max":   "xhigh",
		"gpt-5.5|max":         "xhigh",
		"gpt-5.5|minimal":     "minimal",
	}
	for input, want := range cases {
		parts := strings.SplitN(input, "|", 2)
		if got := codexNormalizeEffort(parts[0], parts[1]); got != want {
			t.Fatalf("%s = %q, want %q", input, got, want)
		}
	}
	out, model, _, err := prepareCodexBody(map[string]any{
		"model": "gpt-5.3-codex-high", "input": "hi",
	}, "gpt-5.3-codex-high", map[string]any{"id": "c"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if model != "gpt-5.3-codex" || out["model"] != "gpt-5.3-codex" {
		t.Fatalf("effort suffix model: %#v", out["model"])
	}
	reasoning, _ := out["reasoning"].(map[string]any)
	if reasoning["effort"] != "high" {
		t.Fatalf("effort suffix: %#v", reasoning)
	}
}

func TestCodexPeekSSEDetectsTransientErrors(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader("event: response.created\ndata: {\"type\":\"response.created\"}\n\ndata: {\"error\":{\"message\":\"Selected model is at capacity. Please try a different model.\",\"code\":\"model_at_capacity\"}}\n\n"))
	prefix, matched, account, message := codexPeekSSE(reader)
	if matched == "" || !account || !strings.Contains(message, "at capacity") || len(prefix) == 0 {
		t.Fatalf("capacity: %q %v %q %d", matched, account, message, len(prefix))
	}

	reader = bufio.NewReader(strings.NewReader("event: error\ndata: {\"type\":\"error\",\"error\":{\"code\":\"server_is_overloaded\",\"message\":\"Please retry\"}}\n\n"))
	prefix, matched, account, message = codexPeekSSE(reader)
	if matched != "server_is_overloaded" || account || message != "Please retry" || len(prefix) == 0 {
		t.Fatalf("overloaded: %q %v %q %d", matched, account, message, len(prefix))
	}

	reader = bufio.NewReader(strings.NewReader("event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hi\"}\n\n"))
	prefix, matched, _, _ = codexPeekSSE(reader)
	if matched != "" || !strings.Contains(string(prefix), "output_text") {
		t.Fatalf("user output should stop peek: %q %q", matched, prefix)
	}
}

func TestFoldResponsesStream(t *testing.T) {
	raw := strings.Join([]string{
		`event: response.created`,
		`data: {"type":"response.created","response":{"id":"resp_1","status":"in_progress"}}`,
		``,
		`event: response.completed`,
		`data: {"type":"response.completed","response":{"id":"resp_1","object":"response","status":"completed","output":[{"type":"message","content":[{"type":"output_text","text":"olá"}]}]}}`,
		``,
		`data: [DONE]`,
	}, "\n")
	out, err := foldResponsesStream([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if out["id"] != "resp_1" || out["object"] != "response" {
		t.Fatalf("folded: %#v", out)
	}

	_, err = foldResponsesStream([]byte("data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"message\":\"boom\"}}}\n"))
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("failed: %v", err)
	}
}

func TestSendCodexOpenAIClientStreamsAndFolds(t *testing.T) {
	upstream := "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\",\"status\":\"in_progress\"}}\n\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"olá\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"status\":\"completed\",\"usage\":{\"input_tokens\":12,\"output_tokens\":3}}}\n\n" +
		"data: [DONE]\n\n"
	for _, stream := range []bool{true, false} {
		t.Run(map[bool]string{true: "stream", false: "json"}[stream], func(t *testing.T) {
			db, key := codexTestConnection(t)
			if _, err := db.Exec(`CREATE TABLE usageHistory(id INTEGER PRIMARY KEY, timestamp TEXT,provider TEXT,model TEXT,connectionId TEXT,apiKey TEXT,endpoint TEXT,promptTokens INTEGER,completionTokens INTEGER,cost REAL,status TEXT,tokens TEXT,meta TEXT); CREATE TABLE usageDaily(dateKey TEXT PRIMARY KEY,data TEXT); CREATE TABLE _meta(key TEXT PRIMARY KEY,value TEXT);`); err != nil {
				t.Fatal(err)
			}
			requests := 0
			send := &TranslatorSend{Connections: database.Connections{DB: db}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				requests++
				if r.URL.String() != "https://chatgpt.com/backend-api/codex/responses" {
					t.Fatalf("url: %s", r.URL)
				}
				for header, want := range map[string]string{
					"Authorization":      "Bearer tok",
					"Session_id":         "ws-1",
					"Chatgpt-Account-Id": "ws-1",
					"Originator":         "codex_cli_rs",
					"Accept":             "text/event-stream",
				} {
					if got := r.Header.Get(header); got != want {
						t.Fatalf("%s = %q, want %q", header, got, want)
					}
				}
				var body map[string]any
				encoded, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(encoded, &body); err != nil {
					t.Fatal(err)
				}
				if body["model"] != "gpt-5.6-sol" || body["stream"] != true || body["store"] != false {
					t.Fatalf("body: %s", encoded)
				}
				if !strings.Contains(ttsString(body["instructions"]), "You are Codex") {
					t.Fatalf("instructions: %s", encoded)
				}
				input, _ := body["input"].([]any)
				if len(input) == 0 || input[0].(map[string]any)["role"] != "user" {
					t.Fatalf("input: %s", encoded)
				}
				return &http.Response{
					StatusCode: 200, Header: http.Header{}, // Real Codex upstream omits Content-Type.
					Body: io.NopCloser(strings.NewReader(upstream)),
				}, nil
			})}}
			gateway := &Gateway{DB: db, Send: send}
			payload := `{"model":"cx/gpt-5.6-sol","messages":[{"role":"user","content":"hi"}],"stream":` + map[bool]string{true: "true", false: "false"}[stream] + `}`
			request := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(payload))
			request.Header.Set("Authorization", "Bearer "+key)
			w := httptest.NewRecorder()
			gateway.OpenAIChat(w, request)
			var usageCount, promptTokens, completionTokens int
			if err := db.QueryRow(`SELECT count(*),coalesce(sum(promptTokens),0),coalesce(sum(completionTokens),0) FROM usageHistory`).Scan(&usageCount, &promptTokens, &completionTokens); err != nil {
				t.Fatal(err)
			}
			if usageCount != 1 || promptTokens != 12 || completionTokens != 3 {
				t.Fatalf("codex usage missing: count=%d prompt=%d completion=%d", usageCount, promptTokens, completionTokens)
			}
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
		})
	}
}

func TestSendCodexResponsesClientPassthroughAndFold(t *testing.T) {
	upstream := "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_9\",\"status\":\"in_progress\"}}\n\n" +
		"event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"olá\"}\n\n" +
		"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_9\",\"object\":\"response\",\"status\":\"completed\"}}\n\n"
	for _, stream := range []bool{true, false} {
		t.Run(map[bool]string{true: "stream", false: "json"}[stream], func(t *testing.T) {
			db, key := codexTestConnection(t)
			send := &TranslatorSend{Connections: database.Connections{DB: db}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: 200, Header: http.Header{}, // Real Codex upstream omits Content-Type.
					Body: io.NopCloser(strings.NewReader(upstream)),
				}, nil
			})}}
			gateway := &Gateway{DB: db, Send: send}
			payload := `{"model":"cx/gpt-5.6-sol","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hi"}]}],"stream":` + map[bool]string{true: "true", false: "false"}[stream] + `}`
			request := httptest.NewRequest("POST", "/api/v1/responses", strings.NewReader(payload))
			request.Header.Set("Authorization", "Bearer "+key)
			w := httptest.NewRecorder()
			gateway.Responses(w, request)
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if stream {
				if w.Body.String() != upstream {
					t.Fatalf("responses passthrough changed the stream:\n%s", w.Body.String())
				}
				return
			}
			var response map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("json: %v %s", err, w.Body.String())
			}
			if response["id"] != "resp_9" || response["object"] != "response" {
				t.Fatalf("folded response: %s", w.Body.String())
			}
		})
	}
}

func TestSendCodexCompactUsesCompactEndpoint(t *testing.T) {
	db, key := codexTestConnection(t)
	requested := ""
	send := &TranslatorSend{Connections: database.Connections{DB: db}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		requested = r.URL.String()
		return &http.Response{
			StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_c\",\"object\":\"response\"}}\n\n")),
		}, nil
	})}}
	gateway := &Gateway{DB: db, Send: send}
	request := httptest.NewRequest("POST", "/api/v1/responses/compact", strings.NewReader(`{"model":"cx/gpt-5.6-sol","input":[{"type":"message","role":"user","content":"hi"}]}`))
	request.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	gateway.Responses(w, request)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if requested != "https://chatgpt.com/backend-api/codex/responses/compact" {
		t.Fatalf("compact url: %s", requested)
	}
}

func TestSendCodexCapacityBecomes503(t *testing.T) {
	db, key := codexTestConnection(t)
	upstream := "data: {\"type\":\"error\",\"error\":{\"code\":\"model_at_capacity\",\"message\":\"Selected model is at capacity. Please try a different model.\"}}\n\n"
	send := &TranslatorSend{Connections: database.Connections{DB: db}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(upstream)),
		}, nil
	})}}
	gateway := &Gateway{DB: db, Send: send}
	request := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"cx/gpt-5.6-sol","messages":[{"role":"user","content":"hi"}]}`))
	request.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	gateway.OpenAIChat(w, request)
	if w.Code != 503 || !strings.Contains(w.Body.String(), "at capacity") || !strings.Contains(w.Body.String(), "service_unavailable") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestSendCodexRetriesOverloaded(t *testing.T) {
	previous := codexRetryDelay
	codexRetryDelay = 0
	t.Cleanup(func() { codexRetryDelay = previous })

	db, key := codexTestConnection(t)
	requests := 0
	send := &TranslatorSend{Connections: database.Connections{DB: db}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		requests++
		body := "data: {\"type\":\"error\",\"error\":{\"code\":\"server_is_overloaded\",\"message\":\"Please retry\"}}\n\n"
		if requests > 1 {
			body = "data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_2\",\"status\":\"completed\"}}\n\n"
		}
		return &http.Response{
			StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}},
			Body: io.NopCloser(strings.NewReader(body)),
		}, nil
	})}}
	gateway := &Gateway{DB: db, Send: send}
	request := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"model":"cx/gpt-5.6-sol","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	request.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	gateway.OpenAIChat(w, request)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"content":"ok"`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	if requests != 2 {
		t.Fatalf("upstream calls: %d", requests)
	}
}

func TestCodexInstructionsEmbedded(t *testing.T) {
	if len(codexDefaultInstructions) < 1000 || !strings.Contains(codexDefaultInstructions, "You are Codex") {
		t.Fatalf("instructions not embedded: %d bytes", len(codexDefaultInstructions))
	}
}

func TestCodexPrefetchImagesConvertsDataURIsAndRejectsLocalhost(t *testing.T) {
	body := map[string]any{"input": []any{map[string]any{
		"type": "message", "role": "user",
		"content": []any{
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,aGVsbG8=", "detail": "high"}},
			map[string]any{"type": "image_url", "image_url": map[string]any{"url": "http://127.0.0.1/secret.png"}},
		},
	}}}
	codexPrefetchImages(context.Background(), body)
	input, _ := body["input"].([]any)
	content, _ := input[0].(map[string]any)["content"].([]any)
	inline, _ := content[0].(map[string]any)
	if inline["type"] != "input_image" || inline["image_url"] != "data:image/png;base64,aGVsbG8=" || inline["detail"] != "high" {
		t.Fatalf("inline image: %#v", inline)
	}
	blocked, _ := content[1].(map[string]any)
	if blocked["image_url"] != "http://127.0.0.1/secret.png" {
		t.Fatalf("blocked image should keep the original URL: %#v", blocked)
	}
}

func TestSendCodexRejectsClaudeClients(t *testing.T) {
	db, key := codexTestConnection(t)
	send := &TranslatorSend{Connections: database.Connections{DB: db}}
	gateway := &Gateway{DB: db, Send: send}
	request := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(`{"model":"cx/gpt-5.6-sol","messages":[{"role":"user","content":"hi"}]}`))
	request.Header.Set("Authorization", "Bearer "+key)
	w := httptest.NewRecorder()
	gateway.OpenAIChat(w, request)
	if w.Code != 501 || !strings.Contains(w.Body.String(), "openai-responses") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

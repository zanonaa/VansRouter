package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func toolGuardTestBody(messages []any) map[string]any {
	return map[string]any{
		"model":    "zn-glm",
		"stream":   false,
		"messages": messages,
		"tools": []any{
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": "subagent",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"agent":       map[string]any{"type": "string"},
							"description": map[string]any{"type": "string"},
							"prompt":      map[string]any{"type": "string"},
							"model":       map[string]any{"type": "string"},
							"sessionID":   map[string]any{"type": "string", "pattern": "^ses"},
							"background":  map[string]any{"type": "boolean"},
						},
						"required": []any{"agent", "description", "prompt"},
					},
				},
			},
			map[string]any{
				"type": "function",
				"function": map[string]any{
					"name": "shell",
					"parameters": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"command": map[string]any{"type": "string"},
							"workdir": map[string]any{"type": "string"},
						},
						"required": []any{"command"},
					},
				},
			},
		},
	}
}

func toolGuardRunJSON(t *testing.T, body map[string]any, response map[string]any) map[string]any {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	recorder := httptest.NewRecorder()
	guard := newToolCallGuard(recorder, request, body)
	guard.Header().Set("Content-Type", "application/json")
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if _, err = guard.Write(raw); err != nil {
		t.Fatalf("write: %v", err)
	}
	guard.Close()
	var out map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal output %q: %v", recorder.Body.String(), err)
	}
	return out
}

func toolGuardArgs(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	choices, _ := body["choices"].([]any)
	if len(choices) == 0 {
		t.Fatalf("no choices in %#v", body)
	}
	message, _ := choices[0].(map[string]any)["message"].(map[string]any)
	calls, _ := message["tool_calls"].([]any)
	if len(calls) == 0 {
		t.Fatalf("no tool_calls in %#v", body)
	}
	function, _ := calls[0].(map[string]any)["function"].(map[string]any)
	raw := ttsString(function["arguments"])
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatalf("invalid arguments %q: %v", raw, err)
	}
	return args
}

func toolGuardJSONResponse(name, args string) map[string]any {
	return map[string]any{
		"id":     "chatcmpl-test",
		"object": "chat.completion",
		"model":  "glm-5.3",
		"choices": []any{
			map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": nil,
					"tool_calls": []any{
						map[string]any{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      name,
								"arguments": args,
							},
						},
					},
				},
				"finish_reason": "tool_calls",
			},
		},
	}
}

func TestToolGuardDropsUngroundedReferences(t *testing.T) {
	body := toolGuardTestBody([]any{
		map[string]any{"role": "user", "content": "Implemente o Token Saver em um subagente."},
	})
	args := toolGuardArgs(t, toolGuardRunJSON(t, body, toolGuardJSONResponse("subagent",
		`{"agent":"explore","background":false,"description":"Implementar Token Saver","model":"auto","prompt":"Implemente","sessionID":"ses_new8"}`)))
	if _, ok := args["sessionID"]; ok {
		t.Fatalf("sessionID inventado não foi removido: %#v", args)
	}
	if _, ok := args["model"]; ok {
		t.Fatalf("model placeholder não foi removido: %#v", args)
	}
	if args["agent"] != "explore" || args["description"] != "Implementar Token Saver" || args["prompt"] != "Implemente" {
		t.Fatalf("campos obrigatórios alterados: %#v", args)
	}
	if args["background"] != false {
		t.Fatalf("background deveria ser preservado: %#v", args)
	}
}

func TestToolGuardKeepsGroundedReferences(t *testing.T) {
	body := toolGuardTestBody([]any{
		map[string]any{"role": "user", "content": "Continue o subagente ses_abc12345def usando o modelo ia-gw-zanona/zn-gpt."},
		map[string]any{"role": "tool", "content": "Subagent spawned (sessionID: ses_abc12345def)"},
	})
	args := toolGuardArgs(t, toolGuardRunJSON(t, body, toolGuardJSONResponse("subagent",
		`{"agent":"general","description":"Continuar","prompt":"Siga","model":"ia-gw-zanona/zn-gpt","sessionID":"ses_abc12345def"}`)))
	if args["sessionID"] != "ses_abc12345def" {
		t.Fatalf("sessionID real foi removido: %#v", args)
	}
	if args["model"] != "ia-gw-zanona/zn-gpt" {
		t.Fatalf("model pedido explicitamente foi removido: %#v", args)
	}
}

func TestToolGuardIgnoresNonReferenceOptional(t *testing.T) {
	body := toolGuardTestBody([]any{
		map[string]any{"role": "user", "content": "Liste os arquivos."},
	})
	args := toolGuardArgs(t, toolGuardRunJSON(t, body, toolGuardJSONResponse("shell",
		`{"command":"ls","workdir":"/tmp/nao-citado-no-prompt"}`)))
	if args["workdir"] != "/tmp/nao-citado-no-prompt" {
		t.Fatalf("workdir opcional não é referência e deveria passar: %#v", args)
	}
}

func TestToolGuardDropsEmptyOptionalValues(t *testing.T) {
	body := toolGuardTestBody([]any{
		map[string]any{"role": "user", "content": "Delegue."},
	})
	args := toolGuardArgs(t, toolGuardRunJSON(t, body, toolGuardJSONResponse("subagent",
		`{"agent":"explore","description":"x","prompt":"y","model":"","sessionID":null}`)))
	if _, ok := args["model"]; ok {
		t.Fatalf("model vazio deveria ser removido: %#v", args)
	}
	if _, ok := args["sessionID"]; ok {
		t.Fatalf("sessionID nulo deveria ser removido: %#v", args)
	}
}

func TestToolGuardDoesNotGroundOnToolSchema(t *testing.T) {
	body := toolGuardTestBody([]any{
		map[string]any{"role": "user", "content": "Delegue."},
	})
	tools := body["tools"].([]any)
	function := tools[0].(map[string]any)["function"].(map[string]any)
	function["description"] = "Exemplo: sessionID ses_new8 (não é uma sessão real)."
	args := toolGuardArgs(t, toolGuardRunJSON(t, body, toolGuardJSONResponse("subagent",
		`{"agent":"explore","description":"x","prompt":"y","sessionID":"ses_new8"}`)))
	if _, ok := args["sessionID"]; ok {
		t.Fatalf("descrição do schema não deve ancorar IDs: %#v", args)
	}
}

func TestToolGuardBypassHeader(t *testing.T) {
	body := toolGuardTestBody([]any{
		map[string]any{"role": "user", "content": "Delegue."},
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	request.Header.Set(toolGuardHeader, "off")
	recorder := httptest.NewRecorder()
	guard := newToolCallGuard(recorder, request, body)
	guard.Header().Set("Content-Type", "application/json")
	raw, _ := json.Marshal(toolGuardJSONResponse("subagent",
		`{"agent":"explore","description":"x","prompt":"y","sessionID":"ses_new8"}`))
	guard.Write(raw)
	guard.Close()
	var out map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	args := toolGuardArgs(t, out)
	if args["sessionID"] != "ses_new8" {
		t.Fatalf("bypass deveria manter a resposta intacta: %#v", args)
	}
}

func TestToolGuardStreamReassemblesToolCalls(t *testing.T) {
	body := toolGuardTestBody([]any{
		map[string]any{"role": "user", "content": "Delegue a implementação."},
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	recorder := httptest.NewRecorder()
	guard := newToolCallGuard(recorder, request, body)
	guard.Header().Set("Content-Type", "text/event-stream")

	chunks := []string{
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"glm-5.3","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"glm-5.3","choices":[{"index":0,"delta":{"content":"Vou delegar."},"finish_reason":null}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"glm-5.3","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"subagent","arguments":"{\"agent\":\"exp"}}]},"finish_reason":null}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"glm-5.3","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"lore\",\"description\":\"Token Saver\",\"prompt\":\"Implemente\",\"sessionID\":\"ses_new8\",\"model\":\"auto\"}"}}]},"finish_reason":null}]}`,
		`{"id":"chatcmpl-1","object":"chat.completion.chunk","created":1700000000,"model":"glm-5.3","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
	}
	for _, chunk := range chunks {
		if _, err := guard.Write([]byte("data: " + chunk + "\n\n")); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if _, err := guard.Write([]byte("data: [DONE]\n\n")); err != nil {
		t.Fatalf("write done: %v", err)
	}
	guard.Close()

	output := recorder.Body.String()
	if !strings.Contains(output, "Vou delegar.") {
		t.Fatalf("texto deveria passar em streaming: %q", output)
	}
	if !strings.Contains(output, "data: [DONE]") {
		t.Fatalf("DONE ausente: %q", output)
	}

	var assembled string
	var sawFinish bool
	sawSanitized := false
	for _, block := range strings.Split(output, "\n\n") {
		block = strings.TrimSpace(block)
		if !strings.HasPrefix(block, "data: ") || strings.Contains(block, "[DONE]") {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(block, "data: ")), &chunk); err != nil {
			t.Fatalf("chunk inválido %q: %v", block, err)
		}
		for _, rawChoice := range chunk["choices"].([]any) {
			choice := rawChoice.(map[string]any)
			delta, _ := choice["delta"].(map[string]any)
			calls, _ := delta["tool_calls"].([]any)
			for _, rawCall := range calls {
				function, _ := rawCall.(map[string]any)["function"].(map[string]any)
				if args := ttsString(function["arguments"]); args != "" {
					assembled += args
					if !strings.Contains(args, "sessionID") && !strings.Contains(args, "model") {
						if strings.Contains(args, `"agent":"explore"`) {
							sawSanitized = true
						}
					}
				}
			}
			if choice["finish_reason"] == "tool_calls" {
				sawFinish = true
			}
		}
	}
	if !sawSanitized {
		t.Fatalf("tool call sanitizado não foi emitido: %q", output)
	}
	if !sawFinish {
		t.Fatalf("finish_reason deveria ser preservado: %q", output)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(assembled), &args); err != nil {
		t.Fatalf("argumentos remontados inválidos %q: %v", assembled, err)
	}
	if _, ok := args["sessionID"]; ok {
		t.Fatalf("sessionID inventado passou no stream: %#v", args)
	}
	if _, ok := args["model"]; ok {
		t.Fatalf("model placeholder passou no stream: %#v", args)
	}
	if args["agent"] != "explore" || args["prompt"] != "Implemente" {
		t.Fatalf("argumentos válidos perdidos: %#v", args)
	}
}

func TestToolGuardRejectsSessionIDsFromFailedHistory(t *testing.T) {
	for _, source := range []map[string]any{
		{"role": "user", "content": "O erro foi Subagent session not found: ses_new1"},
		{"role": "assistant", "content": "Vou usar sessionID: ses_new1"},
		{"role": "tool", "content": "Subagent session not found: ses_new1"},
	} {
		body := toolGuardTestBody([]any{source})
		args := toolGuardArgs(t, toolGuardRunJSON(t, body, toolGuardJSONResponse("subagent",
			`{"agent":"explore","description":"teste","prompt":"leia","sessionID":"ses_new1"}`)))
		if _, ok := args["sessionID"]; ok {
			t.Fatalf("ID inventado foi autorizado pelo histórico: %#v", source)
		}
	}
}

func TestToolGuardKeepsSuccessfulSubagentResult(t *testing.T) {
	body := toolGuardTestBody([]any{map[string]any{
		"role": "tool", "content": `<subagent sessionID="ses_realABC123" state="completed">Concluído</subagent>`,
	}})
	args := toolGuardArgs(t, toolGuardRunJSON(t, body, toolGuardJSONResponse("subagent",
		`{"agent":"explore","description":"teste","prompt":"continue","sessionID":"ses_realABC123"}`)))
	if args["sessionID"] != "ses_realABC123" {
		t.Fatalf("continuação real perdida: %#v", args)
	}
}

func TestToolGuardNoopWithoutTools(t *testing.T) {
	body := map[string]any{
		"model":    "zn-glm",
		"messages": []any{map[string]any{"role": "user", "content": "oi"}},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	recorder := httptest.NewRecorder()
	guard := newToolCallGuard(recorder, request, body)
	if guard.mode != toolGuardPass {
		t.Fatalf("sem tools o guard deve ser passthrough, veio %v", guard.mode)
	}
	guard.Header().Set("Content-Type", "application/json")
	guard.Write([]byte(`{"ok":true}`))
	guard.Close()
	if recorder.Body.String() != `{"ok":true}` {
		t.Fatalf("passthrough alterado: %q", recorder.Body.String())
	}
}

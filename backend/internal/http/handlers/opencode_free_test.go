package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Executor do free tier do OpenCode (porta de open-sse/executors/opencode.js;
// contrato confirmado por github.com/tcatarina/cpa-ocfree).

func TestOpenCodeSessionIDShape(t *testing.T) {
	session := openCodeSessionID()
	if !strings.HasPrefix(session, "ses_") || len(session) != 30 {
		t.Fatalf("shape %q", session)
	}
	// O hex da sessão carrega ~(ms<<12): bit 47 do campo deve ser 1.
	hex := session[4:16]
	if hex[0] < '8' {
		t.Fatalf("sessão não invertida: %q", hex)
	}
	for _, c := range hex {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Fatalf("hex inválido %q", hex)
		}
	}
	for _, c := range session[16:] {
		if !strings.ContainsRune(openCodeBase62, c) {
			t.Fatalf("base62 inválido %q", session[16:])
		}
	}
}

func TestOpenCodeRequestIDShape(t *testing.T) {
	request := openCodeRequestID()
	if !strings.HasPrefix(request, "msg_") || len(request) != 30 {
		t.Fatalf("shape %q", request)
	}
}

func TestOpenCodeHeadersReuseClientIDs(t *testing.T) {
	canonicalSession := "ses_" + strings.Repeat("f", 12) + strings.Repeat("A", 14)
	canonicalRequest := "msg_" + strings.Repeat("0", 12) + strings.Repeat("B", 14)
	client := http.Header{}
	client.Set("x-opencode-session", canonicalSession)
	client.Set("x-opencode-request", canonicalRequest)
	client.Set("x-opencode-client", "agent")
	headers := openCodeHeaders(client, "public")
	if headers["x-opencode-session"] != canonicalSession {
		t.Fatalf("session %q", headers["x-opencode-session"])
	}
	if headers["x-opencode-request"] != canonicalRequest {
		t.Fatalf("request %q", headers["x-opencode-request"])
	}
	if headers["x-opencode-client"] != "agent" || headers["x-opencode-project"] != "global" {
		t.Fatalf("headers %v", headers)
	}
	if headers["User-Agent"] != openCodeUserAgent || headers["Authorization"] != "Bearer public" {
		t.Fatalf("ua/auth %v", headers)
	}
	// IDs malformados do cliente são substituídos, não repassados.
	bad := openCodeHeaders(http.Header{"x-opencode-session": {"abc"}}, "")
	if bad["x-opencode-session"] == "abc" {
		t.Fatal("aceitou session malformado")
	}
}

func TestOpenCodeFingerprintRenamesAndAppendsDecoys(t *testing.T) {
	body := map[string]any{
		"tools": []any{
			map[string]any{"type": "function", "function": map[string]any{"name": "Bash", "parameters": map[string]any{"type": "object"}}},
			map[string]any{"type": "function", "function": map[string]any{"name": "bash", "parameters": map[string]any{"type": "object"}}},
			map[string]any{"type": "function", "function": map[string]any{"name": "minha_tool", "parameters": map[string]any{"type": "object"}}},
		},
	}
	renames := openCodeApplyFingerprint(body)
	if renames["bash"] != "Bash" {
		t.Fatalf("renames %v", renames)
	}
	tools := body["tools"].([]any)
	names := []string{}
	for _, raw := range tools {
		tool := raw.(map[string]any)
		function := tool["function"].(map[string]any)
		names = append(names, ttsString(function["name"]))
	}
	joined := strings.Join(names, ",")
	for _, want := range []string{"bash", "minha_tool", "glob", "grep", "read"} {
		if !strings.Contains(","+joined+",", ","+want+",") {
			t.Fatalf("faltou %s em %v", want, names)
		}
	}
	if strings.Count(joined, "bash") != 1 {
		t.Fatalf("duplicata de bash: %v", names)
	}
	if body["tool_choice"] != nil {
		t.Fatalf("tool_choice deveria continuar nil com tools do caller: %v", body["tool_choice"])
	}
	// Decoy carrega a descrição "unavailable" do executor original.
	last := tools[len(tools)-1].(map[string]any)
	function := last["function"].(map[string]any)
	if ttsString(function["description"]) != openCodeDecoyDescription {
		t.Fatalf("decoy %v", function)
	}
}

func TestOpenCodeFingerprintWithoutCallerToolsChoosesNone(t *testing.T) {
	body := map[string]any{}
	renames := openCodeApplyFingerprint(body)
	if renames != nil {
		t.Fatalf("renames %v", renames)
	}
	if body["tool_choice"] != "none" {
		t.Fatalf("tool_choice %v", body["tool_choice"])
	}
	tools := body["tools"].([]any)
	if len(tools) != 4 {
		t.Fatalf("quarteto %d", len(tools))
	}
}

func openCodeSSE(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return "data: " + string(raw)
}

func TestOpenCodeFoldStreamAndRestore(t *testing.T) {
	call1 := map[string]any{"index": 0, "id": "call_1", "function": map[string]any{"name": "bash", "arguments": `{"a"`}}
	call2 := map[string]any{"index": 0, "function": map[string]any{"name": "", "arguments": ":1}"}}
	chunks := []string{
		openCodeSSE(t, map[string]any{"id": "c1", "created": 1, "model": "m",
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": ""}}}}),
		openCodeSSE(t, map[string]any{
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"reasoning_content": "pensando ", "tool_calls": []any{call1}}}}}),
		openCodeSSE(t, map[string]any{
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "ok", "tool_calls": []any{call2}}}}}),
		openCodeSSE(t, map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": "tool_calls"}},
			"usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 5, "total_tokens": 8}}),
		"data: [DONE]",
		"",
	}
	completion, err := openCodeFoldStream(strings.NewReader(strings.Join(chunks, "\n")), "m", map[string]string{"bash": "Bash"})
	if err != nil {
		t.Fatal(err)
	}
	choice := completion["choices"].([]any)[0].(map[string]any)
	message := choice["message"].(map[string]any)
	if message["content"] != "ok" || message["reasoning_content"] != "pensando " {
		t.Fatalf("message %v", message)
	}
	call := message["tool_calls"].([]any)[0].(map[string]any)
	function := call["function"].(map[string]any)
	if function["name"] != "Bash" || function["arguments"] != `{"a":1}` {
		t.Fatalf("call %v", call)
	}
	if choice["finish_reason"] != "tool_calls" {
		t.Fatalf("finish %v", choice["finish_reason"])
	}
	usage := completion["usage"].(map[string]any)
	if usage["total_tokens"] != float64(8) {
		t.Fatalf("usage %v", usage)
	}
	encoded, _ := json.Marshal(completion)
	if strings.Contains(string(encoded), `"bash"`) {
		t.Fatal("nome canônico vazou na resposta")
	}
}

func TestOpenCodeFoldStreamSurfacesMidStreamError(t *testing.T) {
	stream := "data: {\"error\":\"boom\"}\n\n"
	if _, err := openCodeFoldStream(strings.NewReader(stream), "m", nil); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("erro do stream engolido: %v", err)
	}
}

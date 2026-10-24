package handlers

// Executor do free tier do OpenCode Zen (porta de open-sse/executors/opencode.js
// e utils/opencodeFingerprint.js, confirmada pelo contrato documentado em
// github.com/tcatarina/cpa-ocfree):
//
//   - User-Agent opencode >= 1.18; o tier recusa 426 abaixo disso.
//   - x-opencode-session/msg_-request canônicos: ses_/msg_ + 12 hex + 14 base62;
//     o hex da sessão carrega ~(ms<<12) (bit 47 do campo em 1), o do request
//     ms<<12+1 (ascendente).
//   - x-opencode-project: global; x-opencode-client: desktop.
//   - Corpo: stream forçado true (non-stream responde 403) e o quarteto
//     bash/glob/grep/read declarado exatamente uma vez — variantes
//     capitalizadas do cliente são canonicalizadas e membros faltantes
//     entram como decoys "unavailable"; sem tools do cliente, tool_choice none.
//   - Credencial: Bearer public quando a conexão não tem chave (keyed lane
//     usa a própria chave).
//   - Cliente que pediu JSON recebe o stream upstream dobrado de volta em um
//     chat.completion único, com os nomes originais das tools restaurados.

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openCodeUserAgent = "opencode/1.18.31"
const openCodeDecoyDescription = "This tool is currently unavailable and must not be used."

var openCodeFingerprintTools = []string{"bash", "glob", "grep", "read"}

const openCodeBase62 = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

func openCodeTimeHex(value uint64) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 12)
	for i := 0; i < 6; i++ {
		byteVal := byte((value >> uint(40-8*i)) & 0xff)
		out[i*2] = hexDigits[byteVal>>4]
		out[i*2+1] = hexDigits[byteVal&0x0f]
	}
	return string(out)
}

func openCodeRandomPart() string {
	raw := make([]byte, 14)
	_, _ = rand.Read(raw)
	out := make([]byte, 14)
	for i, b := range raw {
		out[i] = openCodeBase62[int(b)%62]
	}
	return string(out)
}

func openCodeSessionID() string {
	ms := uint64(time.Now().UnixMilli())
	return "ses_" + openCodeTimeHex(^(ms << 12)) + openCodeRandomPart()
}

func openCodeRequestID() string {
	ms := uint64(time.Now().UnixMilli())
	return "msg_" + openCodeTimeHex(ms<<12+1) + openCodeRandomPart()
}

// openCodeHeaders monta os headers de identidade; session/request do próprio
// cliente são reaproveitados quando já têm o formato canônico.
func openCodeHeaders(client http.Header, token string) map[string]string {
	headers := map[string]string{
		"Content-Type":       "application/json",
		"User-Agent":         openCodeUserAgent,
		"x-opencode-client":  "desktop",
		"x-opencode-project": "global",
		"x-opencode-session": openCodeSessionID(),
		"x-opencode-request": openCodeRequestID(),
	}
	if session := strings.TrimSpace(client.Get("x-opencode-session")); openCodePlausibleID("ses_", session) {
		headers["x-opencode-session"] = session
	}
	if request := strings.TrimSpace(client.Get("x-opencode-request")); openCodePlausibleID("msg_", request) {
		headers["x-opencode-request"] = request
	}
	if clientTool := strings.TrimSpace(client.Get("x-opencode-client")); clientTool != "" {
		headers["x-opencode-client"] = clientTool
	}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers
}

func openCodePlausibleID(prefix, value string) bool {
	if len(value) != 30 || !strings.HasPrefix(value, prefix) {
		return false
	}
	for _, c := range value[4:16] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}

func openCodeToolName(tool map[string]any) string {
	if name := strings.TrimSpace(ttsString(tool["name"])); name != "" {
		return name
	}
	function, _ := tool["function"].(map[string]any)
	return strings.TrimSpace(ttsString(function["name"]))
}

func openCodeFingerprintKey(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, tool := range openCodeFingerprintTools {
		if tool == lower {
			return lower
		}
	}
	return ""
}

func openCodeDecoyTool(name string) map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        name,
			"description": openCodeDecoyDescription,
			"parameters":  map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}
}

// openCodeApplyFingerprint canonicaliza o quarteto, descarta duplicatas,
// acrescenta decoys dos membros faltantes e ajusta tool_choice. Retorna o mapa
// enviado -> original para a resposta restaurar os nomes do cliente.
func openCodeApplyFingerprint(body map[string]any) map[string]string {
	renames := map[string]string{}
	tools, _ := body["tools"].([]any)
	hadCallerTools := len(tools) > 0
	cleaned := []any{}
	seen := map[string]bool{}
	for _, raw := range tools {
		tool, ok := raw.(map[string]any)
		if !ok {
			cleaned = append(cleaned, raw)
			continue
		}
		name := openCodeToolName(tool)
		key := openCodeFingerprintKey(name)
		if key == "" {
			cleaned = append(cleaned, tool)
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		if name == key {
			cleaned = append(cleaned, tool)
			continue
		}
		renames[key] = name
		copied := map[string]any{}
		for k, v := range tool {
			copied[k] = v
		}
		if function, ok := tool["function"].(map[string]any); ok {
			fn := map[string]any{}
			for k, v := range function {
				fn[k] = v
			}
			fn["name"] = key
			copied["function"] = fn
		} else {
			copied["name"] = key
		}
		cleaned = append(cleaned, copied)
	}
	for _, name := range openCodeFingerprintTools {
		if !seen[name] {
			cleaned = append(cleaned, openCodeDecoyTool(name))
		}
	}
	body["tools"] = cleaned
	if choice, ok := body["tool_choice"].(map[string]any); ok && len(renames) > 0 {
		if name := ttsString(choice["name"]); renames[name] != "" {
			choice["name"] = openCodeFingerprintKey(name)
		} else if function, ok := choice["function"].(map[string]any); ok {
			if name := ttsString(function["name"]); renames[name] != "" {
				function["name"] = openCodeFingerprintKey(name)
			}
		}
	}
	if !hadCallerTools && body["tool_choice"] == nil {
		body["tool_choice"] = "none"
	}
	if len(renames) == 0 {
		return nil
	}
	return renames
}

// openCodeRestoreNames devolve ao cliente os nomes originais nas tool_calls
// (delta e message), na ida e na volta do stream dobrado.
func openCodeRestoreNames(payload map[string]any, renames map[string]string) {
	if len(renames) == 0 || payload == nil {
		return
	}
	choices, _ := payload["choices"].([]any)
	for _, raw := range choices {
		choice, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		for _, holder := range []string{"delta", "message"} {
			message, ok := choice[holder].(map[string]any)
			if !ok {
				continue
			}
			calls, _ := message["tool_calls"].([]any)
			for _, rawCall := range calls {
				call, ok := rawCall.(map[string]any)
				if !ok {
					continue
				}
				function, _ := call["function"].(map[string]any)
				if function == nil {
					continue
				}
				if original := renames[ttsString(function["name"])]; original != "" {
					function["name"] = original
				}
			}
		}
	}
}

// openCodeFoldStream agrega um SSE OpenAI em um único chat.completion; erro no
// meio do stream é devolvido como erro, nunca como completion vazio.
func openCodeFoldStream(reader io.Reader, model string, renames map[string]string) (map[string]any, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)
	message := map[string]any{"role": "assistant", "content": ""}
	content, reasoning := "", ""
	toolCalls := map[int]map[string]any{}
	finish := any(nil)
	usage := any(nil)
	id, created := "", any(nil)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk map[string]any
		if json.Unmarshal([]byte(data), &chunk) != nil {
			continue
		}
		if failure := chunk["error"]; failure != nil {
			return nil, fmt.Errorf("upstream stream error: %s", ttsString(failure))
		}
		if value := ttsString(chunk["id"]); value != "" {
			id = value
		}
		if value, ok := chunk["created"]; ok {
			created = value
		}
		if value, ok := chunk["usage"].(map[string]any); ok && len(value) > 0 {
			usage = value
		}
		choices, _ := chunk["choices"].([]any)
		for _, raw := range choices {
			choice, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if value := ttsString(choice["finish_reason"]); value != "" {
				finish = value
			}
			delta, _ := choice["delta"].(map[string]any)
			if delta == nil {
				continue
			}
			content += ttsString(delta["content"])
			reasoning += ttsString(firstNonEmpty(delta["reasoning_content"], delta["reasoning"]))
			calls, _ := delta["tool_calls"].([]any)
			for _, rawCall := range calls {
				call, ok := rawCall.(map[string]any)
				if !ok {
					continue
				}
				index := int(pxpipeNum(call["index"]))
				entry := toolCalls[index]
				if entry == nil {
					entry = map[string]any{"id": ttsString(call["id"]), "type": "function", "function": map[string]any{"name": "", "arguments": ""}}
					toolCalls[index] = entry
				}
				if value := ttsString(call["id"]); value != "" {
					entry["id"] = value
				}
				function, _ := call["function"].(map[string]any)
				target := entry["function"].(map[string]any)
				if value := ttsString(function["name"]); value != "" {
					target["name"] = value
				}
				target["arguments"] = ttsString(target["arguments"]) + ttsString(function["arguments"])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if content != "" {
		message["content"] = content
	}
	if reasoning != "" {
		message["reasoning_content"] = reasoning
	}
	if len(toolCalls) > 0 {
		ordered := make([]any, 0, len(toolCalls))
		for i := 0; i < len(toolCalls); i++ {
			if entry, ok := toolCalls[i]; ok {
				ordered = append(ordered, entry)
			}
		}
		message["tool_calls"] = ordered
	}
	if finish == nil {
		finish = "stop"
	}
	completion := map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": created,
		"model":   model,
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}},
	}
	if usage != nil {
		completion["usage"] = usage
	}
	openCodeRestoreNames(completion, renames)
	return completion, nil
}

package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// bufferResponseWriter accumulates converted SSE without committing headers.
// It lets the forced-stream providers (grok-cli/codex) devolverem um único
// chat.completion quando o cliente pediu JSON (paridade com o
// handleForcedSSEToJson da origem).
type bufferResponseWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferResponseWriter) Header() http.Header {
	if b.header == nil {
		b.header = http.Header{}
	}
	return b.header
}
func (b *bufferResponseWriter) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}
func (b *bufferResponseWriter) Write(data []byte) (int, error) {
	if b.status == 0 {
		b.status = 200
	}
	return b.body.Write(data)
}
func (b *bufferResponseWriter) Flush() {}

// foldOpenAIStream porta parseSSEToOpenAIResponse: acumula os chunks OpenAI
// (texto, reasoning e tool calls incrementais) em uma única completion.
func foldOpenAIStream(raw []byte, fallbackModel string) (map[string]any, error) {
	content := strings.Builder{}
	reasoning := strings.Builder{}
	tools := map[int]map[string]any{}
	finish := "stop"
	var usage map[string]any
	id, model := "", fallbackModel
	created := time.Now().Unix()
	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return nil, fmt.Errorf("Invalid provider SSE JSON: %w", err)
		}
		if chunk["error"] != nil {
			return nil, fmt.Errorf("Provider stream returned an error: %v", chunk["error"])
		}
		count++
		if id == "" {
			if value := ttsString(chunk["id"]); value != "" {
				id = value
			}
		}
		if value := ttsString(chunk["model"]); value != "" {
			model = value
		}
		if value, ok := chunk["created"].(float64); ok && value > 0 {
			created = int64(value)
		}
		if rawUsage, ok := chunk["usage"].(map[string]any); ok {
			usage = rawUsage
		}
		choices, _ := chunk["choices"].([]any)
		for _, rawChoice := range choices {
			choice, _ := rawChoice.(map[string]any)
			if choice == nil {
				continue
			}
			delta, _ := choice["delta"].(map[string]any)
			if delta != nil {
				if text, ok := delta["content"].(string); ok {
					content.WriteString(text)
				}
				if text, ok := delta["reasoning_content"].(string); ok {
					reasoning.WriteString(text)
				}
				calls, _ := delta["tool_calls"].([]any)
				for _, rawCall := range calls {
					call, _ := rawCall.(map[string]any)
					index := int(pxpipeNum(call["index"]))
					entry := tools[index]
					if entry == nil {
						entry = map[string]any{"id": "", "type": "function", "function": map[string]any{"name": "", "arguments": ""}}
						tools[index] = entry
					}
					if value := ttsString(call["id"]); value != "" {
						entry["id"] = value
					}
					function, _ := call["function"].(map[string]any)
					target, _ := entry["function"].(map[string]any)
					if value := ttsString(function["name"]); value != "" {
						target["name"] = ttsString(target["name"]) + value
					}
					if value := ttsString(function["arguments"]); value != "" {
						target["arguments"] = ttsString(target["arguments"]) + value
					}
				}
			}
			if value := ttsString(choice["finish_reason"]); value != "" {
				finish = value
			}
		}
	}
	if count == 0 {
		return nil, fmt.Errorf("Provider returned an empty stream")
	}
	if id == "" {
		id = "chatcmpl-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	message := map[string]any{"role": "assistant", "content": content.String()}
	if reasoning.Len() > 0 {
		message["reasoning_content"] = reasoning.String()
	}
	if len(tools) > 0 {
		ordered := []any{}
		for index := 0; index < len(tools); index++ {
			if call, ok := tools[index]; ok {
				ordered = append(ordered, call)
			}
		}
		message["tool_calls"] = ordered
		if content.Len() == 0 {
			message["content"] = nil
		}
	}
	result := map[string]any{"id": id, "object": "chat.completion", "created": created, "model": model, "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}}}
	if usage != nil {
		result["usage"] = usage
	}
	return result, nil
}

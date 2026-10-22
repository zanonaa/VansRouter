package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// convertedResponse buffers only non-streaming cross-format responses. Streaming
// conversion has a separate contract and must not be labelled as plain JSON.
type convertedResponse struct {
	destination           http.ResponseWriter
	header                http.Header
	status                int
	body                  []byte
	source, target, model string
}

func (c *convertedResponse) Header() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}
func (c *convertedResponse) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}
func (c *convertedResponse) Write(body []byte) (int, error) {
	if c.status == 0 {
		c.status = 200
	}
	if len(c.body)+len(body) > 16<<20 {
		return 0, fmt.Errorf("response exceeds conversion limit")
	}
	c.body = append(c.body, body...)
	return len(body), nil
}
func (c *convertedResponse) finish() {
	if c.status == 0 {
		c.status = 200
	}
	if c.status < 200 || c.status >= 300 {
		for k, v := range c.header {
			c.destination.Header()[k] = v
		}
		c.destination.WriteHeader(c.status)
		c.destination.Write(c.body)
		return
	}
	var native map[string]any
	if err := json.Unmarshal(c.body, &native); err != nil {
		nodeReply(c.destination, 502, map[string]any{"error": map[string]any{"message": "Invalid JSON response from provider"}})
		return
	}
	out, err := convertChatResponse(native, c.target, c.source, c.model)
	if err != nil {
		nodeReply(c.destination, 502, map[string]any{"error": map[string]any{"message": err.Error()}})
		return
	}
	nodeReply(c.destination, 200, out)
}
func convertChatResponse(native map[string]any, from, to, model string) (map[string]any, error) {
	if from == to {
		return native, nil
	}
	message := map[string]any{"role": "assistant", "content": ""}
	finish := any("stop")
	id := native["id"]
	usage := map[string]any{}
	switch from {
	case "openai":
		choices, ok := native["choices"].([]any)
		if !ok || len(choices) == 0 {
			return nil, fmt.Errorf("Provider returned no completion choices")
		}
		choice, _ := choices[0].(map[string]any)
		var valid bool
		message, valid = choice["message"].(map[string]any)
		if !valid {
			return nil, fmt.Errorf("Invalid completion message")
		}
		finish = choice["finish_reason"]
		usage, _ = native["usage"].(map[string]any)
	case "claude":
		blocks, _ := native["content"].([]any)
		content, calls, _, err := claudeContent(blocks)
		if err != nil {
			return nil, err
		}
		message["content"] = collapseSourceText(content)
		if len(calls) > 0 {
			message["tool_calls"] = calls
		}
		switch native["stop_reason"] {
		case "max_tokens":
			finish = "length"
		case "tool_use":
			finish = "tool_calls"
		}
		rawUsage, _ := native["usage"].(map[string]any)
		prompt, completion := pxpipeNum(rawUsage["input_tokens"]), pxpipeNum(rawUsage["output_tokens"])
		usage = map[string]any{"prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": prompt + completion}
	case "gemini":
		candidates, _ := native["candidates"].([]any)
		if len(candidates) == 0 {
			return nil, fmt.Errorf("Provider returned no Gemini candidates")
		}
		candidate, _ := candidates[0].(map[string]any)
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		blocks, calls, _, err := geminiParts(parts)
		if err != nil {
			return nil, err
		}
		message["content"] = collapseSourceText(blocks)
		if len(calls) > 0 {
			message["tool_calls"] = calls
			finish = "tool_calls"
		}
		if candidate["finishReason"] == "MAX_TOKENS" {
			finish = "length"
		}
		rawUsage, _ := native["usageMetadata"].(map[string]any)
		usage = map[string]any{"prompt_tokens": rawUsage["promptTokenCount"], "completion_tokens": rawUsage["candidatesTokenCount"], "total_tokens": rawUsage["totalTokenCount"]}
		id = native["responseId"]
	default:
		return nil, fmt.Errorf("Unsupported response format: %s", from)
	}
	switch to {
	case "openai":
		return map[string]any{"id": id, "object": "chat.completion", "model": model, "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}}, "usage": usage}, nil
	case "claude":
		content, err := targetContent(message["content"], "claude")
		if err != nil {
			return nil, err
		}
		if calls, ok := message["tool_calls"].([]any); ok {
			for _, raw := range calls {
				call, _ := raw.(map[string]any)
				function, _ := call["function"].(map[string]any)
				var input any
				if err := json.Unmarshal([]byte(ttsString(function["arguments"])), &input); err != nil {
					return nil, err
				}
				content = append(content, map[string]any{"type": "tool_use", "id": call["id"], "name": function["name"], "input": input})
			}
		}
		reason := "end_turn"
		if finish == "length" {
			reason = "max_tokens"
		}
		if finish == "tool_calls" {
			reason = "tool_use"
		}
		return map[string]any{"id": id, "type": "message", "role": "assistant", "model": model, "content": content, "stop_reason": reason, "stop_sequence": nil, "usage": map[string]any{"input_tokens": usage["prompt_tokens"], "output_tokens": usage["completion_tokens"]}}, nil
	case "openai-responses":
		output := []any{}
		if text := ttsString(message["content"]); text != "" {
			output = append(output, map[string]any{"type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}}})
		}
		calls, _ := message["tool_calls"].([]any)
		for _, raw := range calls {
			call, _ := raw.(map[string]any)
			function, _ := call["function"].(map[string]any)
			arguments := ttsString(function["arguments"])
			if !json.Valid([]byte(arguments)) {
				return nil, fmt.Errorf("Responses tool arguments are not valid JSON")
			}
			output = append(output, map[string]any{"type": "function_call", "call_id": call["id"], "name": function["name"], "arguments": arguments, "status": "completed"})
		}
		if len(output) == 0 {
			return nil, fmt.Errorf("Responses output is empty")
		}
		status := "completed"
		result := map[string]any{"id": id, "object": "response", "model": model, "status": status, "output": output, "usage": map[string]any{"input_tokens": usage["prompt_tokens"], "output_tokens": usage["completion_tokens"], "total_tokens": usage["total_tokens"]}}
		if finish == "length" {
			result["status"] = "incomplete"
			result["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("Unsupported client response format: %s", to)
	}
}

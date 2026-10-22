package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// translateTarget converts the OpenAI intermediate into the native request.
// Blocks with no lossless mapping fail explicitly rather than being discarded.
func translateTarget(body map[string]any, format, model string) (map[string]any, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var source map[string]any
	if err = json.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	if format == "openai" {
		source["model"] = model
		return source, nil
	}
	messages, ok := source["messages"].([]any)
	if !ok {
		return nil, fmt.Errorf("messages must be an array")
	}
	if format != "claude" && format != "gemini" {
		return nil, fmt.Errorf("Unsupported target format: %s", format)
	}
	turns := []any{}
	system := []any{}
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Invalid message")
		}
		role := ttsString(message["role"])
		blocks, err := targetContent(message["content"], format)
		if err != nil {
			return nil, err
		}
		if role == "system" || role == "developer" {
			system = append(system, blocks...)
			continue
		}
		if role == "tool" {
			if format == "gemini" {
				return nil, fmt.Errorf("Gemini tool result requires resolved function name")
			}
			blocks = []any{map[string]any{"type": "tool_result", "tool_use_id": message["tool_call_id"], "content": message["content"]}}
			role = "user"
		}
		if calls, ok := message["tool_calls"].([]any); ok {
			for _, rawCall := range calls {
				call, _ := rawCall.(map[string]any)
				function, _ := call["function"].(map[string]any)
				var arguments any = map[string]any{}
				if text, ok := function["arguments"].(string); ok && text != "" {
					if err := json.Unmarshal([]byte(text), &arguments); err != nil {
						return nil, fmt.Errorf("Invalid tool call arguments: %w", err)
					}
				}
				if format == "claude" {
					blocks = append(blocks, map[string]any{"type": "tool_use", "id": call["id"], "name": function["name"], "input": arguments})
				} else {
					blocks = append(blocks, map[string]any{"functionCall": map[string]any{"name": function["name"], "args": arguments}})
				}
			}
		}
		if role != "user" && role != "assistant" {
			return nil, fmt.Errorf("Unsupported message role: %s", role)
		}
		if format == "gemini" {
			if role == "assistant" {
				role = "model"
			}
			turns = append(turns, map[string]any{"role": role, "parts": blocks})
		} else {
			turns = append(turns, map[string]any{"role": role, "content": blocks})
		}
	}
	result := map[string]any{}
	if format == "claude" {
		result = map[string]any{"model": model, "messages": turns, "stream": source["stream"] != false, "max_tokens": 1024}
		if len(system) > 0 {
			result["system"] = system
		}
		for _, key := range []string{"max_tokens", "temperature", "top_p", "top_k", "metadata", "thinking"} {
			if value, ok := source[key]; ok {
				result[key] = value
			}
		}
		if value, ok := source["max_completion_tokens"]; ok {
			result["max_tokens"] = value
		}
		if stop, ok := source["stop"]; ok {
			if text, ok := stop.(string); ok {
				stop = []any{text}
			}
			result["stop_sequences"] = stop
		}
	} else {
		result["contents"] = turns
		if len(system) > 0 {
			result["systemInstruction"] = map[string]any{"parts": system}
		}
		config := map[string]any{}
		for from, to := range map[string]string{"temperature": "temperature", "top_p": "topP", "top_k": "topK", "max_tokens": "maxOutputTokens", "max_completion_tokens": "maxOutputTokens", "stop": "stopSequences"} {
			if value, ok := source[from]; ok {
				if from == "stop" {
					if text, ok := value.(string); ok {
						value = []any{text}
					}
				}
				config[to] = value
			}
		}
		if len(config) > 0 {
			result["generationConfig"] = config
		}
	}
	if tools, ok := source["tools"].([]any); ok && len(tools) > 0 {
		converted := []any{}
		for _, raw := range tools {
			tool, _ := raw.(map[string]any)
			function, _ := tool["function"].(map[string]any)
			if tool["type"] != "function" || function == nil {
				return nil, fmt.Errorf("Unsupported tool type")
			}
			schema := function["parameters"]
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			item := map[string]any{"name": function["name"], "description": firstNonEmpty(function["description"], "")}
			if format == "claude" {
				item["input_schema"] = schema
			} else {
				item["parameters"] = schema
			}
			converted = append(converted, item)
		}
		if format == "claude" {
			result["tools"] = converted
		} else {
			result["tools"] = []any{map[string]any{"functionDeclarations": converted}}
		}
	}
	if choice, ok := source["tool_choice"]; ok {
		if format == "gemini" {
			config, err := geminiToolConfig(choice)
			if err != nil {
				return nil, err
			}
			result["toolConfig"] = config
			return result, nil
		}
		switch value := choice.(type) {
		case string:
			kind := value
			if kind == "required" {
				kind = "any"
			}
			if kind != "auto" && kind != "any" && kind != "none" {
				return nil, fmt.Errorf("Unsupported tool_choice: %s", value)
			}
			result["tool_choice"] = map[string]any{"type": kind}
		case map[string]any:
			function, _ := value["function"].(map[string]any)
			result["tool_choice"] = map[string]any{"type": "tool", "name": function["name"]}
		default:
			return nil, fmt.Errorf("Invalid tool_choice")
		}
	}
	return result, nil
}

func geminiToolConfig(choice any) (map[string]any, error) {
	mode := ""
	allowed := []any{}
	switch value := choice.(type) {
	case string:
		switch value {
		case "auto":
			mode = "AUTO"
		case "none":
			mode = "NONE"
		case "required":
			mode = "ANY"
		default:
			return nil, fmt.Errorf("Unsupported tool_choice: %s", value)
		}
	case map[string]any:
		function, _ := value["function"].(map[string]any)
		name := ttsString(function["name"])
		if value["type"] != "function" || name == "" {
			return nil, fmt.Errorf("Invalid tool_choice")
		}
		mode = "ANY"
		allowed = []any{name}
	default:
		return nil, fmt.Errorf("Invalid tool_choice")
	}
	config := map[string]any{"mode": mode}
	if len(allowed) > 0 {
		config["allowedFunctionNames"] = allowed
	}
	return map[string]any{"functionCallingConfig": config}, nil
}

func targetContent(content any, format string) ([]any, error) {
	if content == nil {
		return []any{}, nil
	}
	if text, ok := content.(string); ok {
		if format == "claude" {
			return []any{map[string]any{"type": "text", "text": text}}, nil
		}
		return []any{map[string]any{"text": text}}, nil
	}
	entries, ok := content.([]any)
	if !ok {
		return nil, fmt.Errorf("Invalid message content")
	}
	result := []any{}
	for _, raw := range entries {
		block, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Invalid content block")
		}
		switch block["type"] {
		case "text":
			if format == "claude" {
				result = append(result, map[string]any{"type": "text", "text": block["text"]})
			} else {
				result = append(result, map[string]any{"text": block["text"]})
			}
		case "image_url":
			image, _ := block["image_url"].(map[string]any)
			url := ttsString(image["url"])
			if strings.HasPrefix(url, "data:") {
				parts := strings.SplitN(strings.TrimPrefix(url, "data:"), ",", 2)
				if len(parts) != 2 || !strings.HasSuffix(parts[0], ";base64") {
					return nil, fmt.Errorf("Invalid inline image")
				}
				mime := strings.TrimSuffix(parts[0], ";base64")
				if format == "claude" {
					result = append(result, map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": mime, "data": parts[1]}})
				} else {
					result = append(result, map[string]any{"inlineData": map[string]any{"mimeType": mime, "data": parts[1]}})
				}
			} else if format == "claude" {
				result = append(result, map[string]any{"type": "image", "source": map[string]any{"type": "url", "url": url}})
			} else {
				return nil, fmt.Errorf("Gemini remote images require explicit upload")
			}
		default:
			return nil, fmt.Errorf("Unsupported content block: %v", block["type"])
		}
	}
	return result, nil
}

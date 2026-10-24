package handlers

import (
	"encoding/json"
	"fmt"
	"strings"
)

// translateSource converts native conversations into the OpenAI intermediate.
// Unknown blocks fail explicitly, preserving the caller's ability to detect loss.
func translateSource(body map[string]any, format string) (map[string]any, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var source map[string]any
	if err = json.Unmarshal(raw, &source); err != nil {
		return nil, err
	}
	if format == "openai" {
		return source, nil
	}
	if format == "openai-responses" {
		return responsesToIntermediate(source)
	}
	result := map[string]any{}
	for _, key := range []string{"model", "stream", "temperature", "top_p", "top_k", "max_tokens", "metadata"} {
		if value, ok := source[key]; ok {
			result[key] = value
		}
	}
	messages := []any{}
	switch format {
	case "claude":
		if system, ok := source["system"]; ok {
			blocks, _, _, err := claudeContent(system)
			if err != nil {
				return nil, err
			}
			messages = append(messages, map[string]any{"role": "system", "content": collapseSourceText(blocks)})
		}
		turns, ok := source["messages"].([]any)
		if !ok {
			return nil, fmt.Errorf("messages must be an array")
		}
		for _, raw := range turns {
			turn, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("Invalid Claude message")
			}
			blocks, calls, toolResults, err := claudeContent(turn["content"])
			if err != nil {
				return nil, err
			}
			if len(blocks) > 0 || len(calls) > 0 {
				message := map[string]any{"role": turn["role"], "content": collapseSourceText(blocks)}
				if len(calls) > 0 {
					message["tool_calls"] = calls
				}
				messages = append(messages, message)
			}
			messages = append(messages, toolResults...)
		}
		if stop, ok := source["stop_sequences"]; ok {
			result["stop"] = stop
		}
		if tools, ok := source["tools"].([]any); ok {
			converted := []any{}
			for _, raw := range tools {
				tool, ok := raw.(map[string]any)
				if !ok || tool["name"] == nil {
					return nil, fmt.Errorf("Unsupported Claude tool")
				}
				converted = append(converted, map[string]any{"type": "function", "function": map[string]any{"name": tool["name"], "description": tool["description"], "parameters": tool["input_schema"]}})
			}
			result["tools"] = converted
		}
		if choice, ok := source["tool_choice"].(map[string]any); ok {
			switch choice["type"] {
			case "auto":
				result["tool_choice"] = "auto"
			case "any":
				result["tool_choice"] = "required"
			case "none":
				result["tool_choice"] = "none"
			case "tool":
				result["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": choice["name"]}}
			default:
				return nil, fmt.Errorf("Unsupported Claude tool_choice")
			}
		}
	case "gemini":
		if system, ok := source["systemInstruction"].(map[string]any); ok {
			parts, _ := system["parts"].([]any)
			blocks, _, _, err := geminiParts(parts)
			if err != nil {
				return nil, err
			}
			messages = append(messages, map[string]any{"role": "system", "content": collapseSourceText(blocks)})
		}
		turns, ok := source["contents"].([]any)
		if !ok {
			return nil, fmt.Errorf("contents must be an array")
		}
		for _, raw := range turns {
			turn, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("Invalid Gemini content")
			}
			parts, ok := turn["parts"].([]any)
			if !ok {
				return nil, fmt.Errorf("Gemini parts must be an array")
			}
			blocks, calls, results, err := geminiParts(parts)
			if err != nil {
				return nil, err
			}
			role := ttsString(turn["role"])
			if role == "model" {
				role = "assistant"
			}
			if role == "" {
				role = "user"
			}
			if len(blocks) > 0 || len(calls) > 0 {
				message := map[string]any{"role": role, "content": collapseSourceText(blocks)}
				if len(calls) > 0 {
					message["tool_calls"] = calls
				}
				messages = append(messages, message)
			}
			messages = append(messages, results...)
		}
		if config, ok := source["generationConfig"].(map[string]any); ok {
			for from, to := range map[string]string{"temperature": "temperature", "topP": "top_p", "topK": "top_k", "maxOutputTokens": "max_tokens", "stopSequences": "stop"} {
				if value, ok := config[from]; ok {
					result[to] = value
				}
			}
		}
		if tools, ok := source["tools"].([]any); ok {
			converted := []any{}
			for _, raw := range tools {
				tool, _ := raw.(map[string]any)
				declarations, ok := tool["functionDeclarations"].([]any)
				if !ok {
					return nil, fmt.Errorf("Unsupported Gemini tool")
				}
				for _, raw := range declarations {
					function, _ := raw.(map[string]any)
					converted = append(converted, map[string]any{"type": "function", "function": function})
				}
			}
			result["tools"] = converted
		}
	default:
		return nil, fmt.Errorf("Unsupported source format: %s", format)
	}
	result["messages"] = messages
	return result, nil
}

func collapseSourceText(blocks []any) any {
	if len(blocks) == 0 {
		return ""
	}
	texts := []string{}
	for _, raw := range blocks {
		block, _ := raw.(map[string]any)
		if block["type"] != "text" {
			return blocks
		}
		text, ok := block["text"].(string)
		if !ok {
			return blocks
		}
		texts = append(texts, text)
	}
	return strings.Join(texts, "\n")
}

func claudeContent(content any) ([]any, []any, []any, error) {
	if text, ok := content.(string); ok {
		return []any{map[string]any{"type": "text", "text": text}}, nil, nil, nil
	}
	if content == nil {
		return nil, nil, nil, nil
	}
	entries, ok := content.([]any)
	if !ok {
		return nil, nil, nil, fmt.Errorf("Invalid Claude content")
	}
	blocks, calls, results := []any{}, []any{}, []any{}
	for _, raw := range entries {
		block, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, nil, fmt.Errorf("Invalid Claude content block")
		}
		switch block["type"] {
		case "text":
			blocks = append(blocks, map[string]any{"type": "text", "text": block["text"]})
		case "thinking":
			// Extraído pelo chamador quando relevante; nunca rejeita a resposta.
		case "image":
			source, _ := block["source"].(map[string]any)
			imageURL := ""
			switch source["type"] {
			case "base64":
				imageURL = "data:" + ttsString(source["media_type"]) + ";base64," + ttsString(source["data"])
			case "url":
				imageURL = ttsString(source["url"])
			default:
				return nil, nil, nil, fmt.Errorf("Unsupported Claude image source")
			}
			blocks = append(blocks, map[string]any{"type": "image_url", "image_url": map[string]any{"url": imageURL}})
		case "tool_use":
			arguments, err := json.Marshal(block["input"])
			if err != nil {
				return nil, nil, nil, err
			}
			calls = append(calls, map[string]any{"id": block["id"], "type": "function", "function": map[string]any{"name": block["name"], "arguments": string(arguments)}})
		case "tool_result":
			content := block["content"]
			if entries, ok := content.([]any); ok {
				converted, _, _, err := claudeContent(entries)
				if err != nil {
					return nil, nil, nil, err
				}
				content = collapseSourceText(converted)
			}
			results = append(results, map[string]any{"role": "tool", "tool_call_id": block["tool_use_id"], "content": content})
		default:
			return nil, nil, nil, fmt.Errorf("Unsupported Claude content block: %v", block["type"])
		}
	}
	return blocks, calls, results, nil
}

func geminiParts(parts []any) ([]any, []any, []any, error) {
	blocks, calls, results := []any{}, []any{}, []any{}
	for _, raw := range parts {
		part, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, nil, fmt.Errorf("Invalid Gemini part")
		}
		if text, ok := part["text"].(string); ok {
			if part["thought"] == true {
				return nil, nil, nil, fmt.Errorf("Gemini thought part requires reasoning mapping")
			}
			blocks = append(blocks, map[string]any{"type": "text", "text": text})
			continue
		}
		if inline, ok := part["inlineData"].(map[string]any); ok {
			mime := ttsString(inline["mimeType"])
			if !strings.HasPrefix(mime, "image/") {
				return nil, nil, nil, fmt.Errorf("Unsupported Gemini inline media: %s", mime)
			}
			blocks = append(blocks, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + mime + ";base64," + ttsString(inline["data"])}})
			continue
		}
		if function, ok := part["functionCall"].(map[string]any); ok {
			arguments, err := json.Marshal(function["args"])
			if err != nil {
				return nil, nil, nil, err
			}
			id := ttsString(function["id"])
			if id == "" {
				id = "call_" + ttsString(function["name"])
			}
			calls = append(calls, map[string]any{"id": id, "type": "function", "function": map[string]any{"name": function["name"], "arguments": string(arguments)}})
			continue
		}
		if function, ok := part["functionResponse"].(map[string]any); ok {
			encoded, err := json.Marshal(function["response"])
			if err != nil {
				return nil, nil, nil, err
			}
			id := ttsString(function["id"])
			if id == "" {
				id = "call_" + ttsString(function["name"])
			}
			results = append(results, map[string]any{"role": "tool", "tool_call_id": id, "name": function["name"], "content": string(encoded)})
			continue
		}
		return nil, nil, nil, fmt.Errorf("Unsupported Gemini part")
	}
	return blocks, calls, results, nil
}

func responsesToIntermediate(source map[string]any) (map[string]any, error) {
	result := map[string]any{}
	for _, key := range []string{"model", "stream", "temperature", "top_p", "tools", "tool_choice"} {
		if value, ok := source[key]; ok {
			result[key] = value
		}
	}
	if value, ok := source["max_output_tokens"]; ok {
		result["max_tokens"] = value
	}
	messages := []any{}
	if instruction, ok := source["instructions"].(string); ok {
		messages = append(messages, map[string]any{"role": "system", "content": instruction})
	}
	if text, ok := source["input"].(string); ok {
		messages = append(messages, map[string]any{"role": "user", "content": text})
		result["messages"] = messages
		return result, nil
	}
	input, ok := source["input"].([]any)
	if !ok {
		return nil, fmt.Errorf("Responses input must be a string or array")
	}
	for _, raw := range input {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Invalid Responses item")
		}
		kind := ttsString(item["type"])
		switch kind {
		case "", "message":
			message := map[string]any{"role": item["role"], "content": item["content"]}
			if entries, ok := item["content"].([]any); ok {
				blocks := []any{}
				for _, raw := range entries {
					block, _ := raw.(map[string]any)
					switch block["type"] {
					case "input_text", "output_text":
						blocks = append(blocks, map[string]any{"type": "text", "text": block["text"]})
					case "input_image":
						blocks = append(blocks, map[string]any{"type": "image_url", "image_url": map[string]any{"url": block["image_url"]}})
					default:
						return nil, fmt.Errorf("Unsupported Responses content block")
					}
				}
				message["content"] = collapseSourceText(blocks)
			}
			messages = append(messages, message)
		case "function_call":
			messages = append(messages, map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{"id": item["call_id"], "type": "function", "function": map[string]any{"name": item["name"], "arguments": item["arguments"]}}}})
		case "function_call_output":
			messages = append(messages, map[string]any{"role": "tool", "tool_call_id": item["call_id"], "content": item["output"]})
		default:
			return nil, fmt.Errorf("Unsupported Responses item: %s", kind)
		}
	}
	result["messages"] = messages
	return result, nil
}

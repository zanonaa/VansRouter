package handlers

import (
	"encoding/json"
	"fmt"
)

// Porta de open-sse/translator/request/openai-responses.js
// (openaiToOpenAIResponsesRequest). O intermediário OpenAI (chat completions)
// vira o corpo da Responses API usado por grok-cli/codex: system vira
// instructions + developer no input, tool_calls viram function_call, tool
// results viram function_call_output e as tools são achatadas.
const responsesCallIDMax = 64

func clampResponsesCallID(value any) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	if len(text) > responsesCallIDMax {
		return text[:responsesCallIDMax]
	}
	return text
}

func normalizeResponsesToolParameters(params any) any {
	if params == nil {
		return map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if object, ok := params.(map[string]any); ok && object["type"] == "object" && object["properties"] == nil {
		copy := map[string]any{}
		for key, value := range object {
			copy[key] = value
		}
		copy["properties"] = map[string]any{}
		return copy
	}
	return params
}

// responsesReasoningItem reemite o raciocínio de um turno anterior. Em
// backends store=false (Grok CLI/Codex) o encrypted_content é o token de
// continuidade; sem ele o upstream perde o histórico de pensamento.
func responsesReasoningItem(message map[string]any) map[string]any {
	encrypted := ""
	for _, key := range []string{"encrypted_content", "reasoning_encrypted_content"} {
		if text, ok := message[key].(string); ok && text != "" {
			encrypted = text
			break
		}
	}
	if reasoning, ok := message["reasoning"].(map[string]any); ok && encrypted == "" {
		if text, ok := reasoning["encrypted_content"].(string); ok {
			encrypted = text
		}
	}
	summary := ""
	if text, ok := message["reasoning_content"].(string); ok && text != "" {
		summary = text
	} else if text, ok := message["reasoning"].(string); ok && text != "" {
		summary = text
	} else if details, ok := message["reasoning_details"].([]any); ok {
		for _, raw := range details {
			detail, _ := raw.(map[string]any)
			text := ttsString(detail["text"])
			if text == "" {
				text = ttsString(detail["content"])
			}
			if text != "" {
				if summary != "" {
					summary += "\n"
				}
				summary += text
			}
		}
	}
	if encrypted == "" && summary == "" {
		return nil
	}
	item := map[string]any{"type": "reasoning"}
	if summary != "" {
		item["summary"] = []any{map[string]any{"type": "summary_text", "text": summary}}
	}
	if encrypted != "" {
		item["encrypted_content"] = encrypted
	}
	return item
}

func responsesText(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func openAIResponsesTarget(source map[string]any, model string) (map[string]any, error) {
	// Corpo já em Responses (cliente /v1/responses chamando /chat/completions
	// com input[]): passa direto, só ajusta modelo/stream/limite de tokens.
	if source["input"] != nil {
		result := map[string]any{}
		for key, value := range source {
			result[key] = value
		}
		result["model"] = model
		result["stream"] = true
		if result["max_output_tokens"] == nil {
			if value := result["max_completion_tokens"]; value != nil {
				result["max_output_tokens"] = value
			} else if value := result["max_tokens"]; value != nil {
				result["max_output_tokens"] = value
			}
		}
		delete(result, "max_tokens")
		delete(result, "max_completion_tokens")
		return result, nil
	}
	messages, ok := source["messages"].([]any)
	if !ok {
		return nil, fmt.Errorf("messages must be an array")
	}
	result := map[string]any{"model": model, "input": []any{}, "stream": true, "store": false}
	input := []any{}
	hasSystem := false
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Invalid message")
		}
		role := ttsString(message["role"])
		switch role {
		case "system", "developer":
			if !hasSystem {
				result["instructions"] = ""
				if text, ok := message["content"].(string); ok {
					result["instructions"] = text
				}
				hasSystem = true
			}
			text := ""
			if value, ok := message["content"].(string); ok {
				text = value
			} else {
				text = responsesText(message["content"])
			}
			input = append(input, map[string]any{
				"type": "message", "role": "developer",
				"content": []any{map[string]any{"type": "input_text", "text": text}},
			})
		case "user", "assistant":
			if role == "assistant" {
				if item := responsesReasoningItem(message); item != nil {
					input = append(input, item)
				}
			}
			contentType := "input_text"
			if role == "assistant" {
				contentType = "output_text"
			}
			blocks := []any{}
			switch content := message["content"].(type) {
			case string:
				blocks = append(blocks, map[string]any{"type": contentType, "text": content})
			case []any:
				for _, rawBlock := range content {
					block, ok := rawBlock.(map[string]any)
					if !ok {
						return nil, fmt.Errorf("Invalid content block")
					}
					switch block["type"] {
					case "text":
						blocks = append(blocks, map[string]any{"type": contentType, "text": block["text"]})
					case "image_url":
						url, detail := "", "auto"
						switch image := block["image_url"].(type) {
						case string:
							url = image
						case map[string]any:
							url = ttsString(image["url"])
							if value := ttsString(image["detail"]); value != "" {
								detail = value
							}
						}
						blocks = append(blocks, map[string]any{"type": "input_image", "image_url": url, "detail": detail})
					case "input_image":
						blocks = append(blocks, block)
					default:
						text := block["text"]
						if text == nil {
							text = block["content"]
						}
						if text == nil {
							blocks = append(blocks, map[string]any{"type": contentType, "text": responsesText(block)})
							continue
						}
						blocks = append(blocks, map[string]any{"type": contentType, "text": responsesText(text)})
					}
				}
			}
			if len(blocks) > 0 {
				input = append(input, map[string]any{"type": "message", "role": role, "content": blocks})
			}
		}
		if role == "assistant" {
			if calls, ok := message["tool_calls"].([]any); ok {
				for _, rawCall := range calls {
					call, _ := rawCall.(map[string]any)
					function, _ := call["function"].(map[string]any)
					name := ttsString(function["name"])
					if name == "" {
						name = "_unknown"
					}
					arguments := ttsString(function["arguments"])
					if arguments == "" {
						arguments = "{}"
					}
					input = append(input, map[string]any{
						"type": "function_call", "call_id": clampResponsesCallID(call["id"]),
						"name": name, "arguments": arguments,
					})
				}
			}
		}
		if role == "tool" {
			output := ""
			switch content := message["content"].(type) {
			case string:
				output = content
			case []any:
				for _, rawEntry := range content {
					entry, _ := rawEntry.(map[string]any)
					if text, ok := entry["text"].(string); ok {
						output += text
					} else {
						output += responsesText(rawEntry)
					}
				}
			case nil:
			default:
				output = responsesText(content)
			}
			input = append(input, map[string]any{
				"type": "function_call_output", "call_id": clampResponsesCallID(message["tool_call_id"]),
				"output": output,
			})
		}
	}
	result["input"] = input
	if !hasSystem {
		result["instructions"] = ""
	}
	if tools, ok := source["tools"].([]any); ok {
		converted := []any{}
		for _, raw := range tools {
			tool, _ := raw.(map[string]any)
			if tool == nil {
				continue
			}
			if tool["type"] != "function" {
				converted = append(converted, tool)
				continue
			}
			function, _ := tool["function"].(map[string]any)
			if function == nil {
				converted = append(converted, tool)
				continue
			}
			item := map[string]any{
				"type": "function", "name": function["name"],
				"description": ttsString(function["description"]),
				"parameters":  normalizeResponsesToolParameters(function["parameters"]),
			}
			if value, ok := function["strict"]; ok {
				item["strict"] = value
			}
			converted = append(converted, item)
		}
		result["tools"] = converted
	}
	for _, key := range []string{"temperature", "top_p", "reasoning", "service_tier", "prompt_cache_key", "max_output_tokens"} {
		if value, ok := source[key]; ok {
			result[key] = value
		}
	}
	if _, ok := result["max_output_tokens"]; !ok {
		if value, ok := source["max_completion_tokens"]; ok {
			result["max_output_tokens"] = value
		} else if value, ok := source["max_tokens"]; ok {
			result["max_output_tokens"] = value
		}
	}
	if _, ok := result["reasoning"]; !ok {
		if value, ok := source["reasoning_effort"]; ok {
			result["reasoning"] = map[string]any{"effort": value, "summary": "auto"}
		}
	}
	return result, nil
}

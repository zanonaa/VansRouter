package handlers

import (
	"encoding/json"
	"math"
	"net/http"
)

func countValueChars(value any) int {
	switch typed := value.(type) {
	case nil:
		return 0
	case string:
		return len([]rune(typed))
	case float64, bool:
		return len([]rune(jsString(typed)))
	case []any:
		total := 0
		for _, item := range typed {
			total += countValueChars(item)
		}
		return total
	case map[string]any:
		total := 0
		for key, item := range typed {
			total += len([]rune(key)) + countValueChars(item)
		}
		return total
	default:
		return 0
	}
}

func countContentBlockChars(value any) int {
	switch typed := value.(type) {
	case nil:
		return 0
	case string:
		return len([]rune(typed))
	case map[string]any:
		switch typed["type"] {
		case "text":
			return countValueChars(typed["text"])
		case "tool_use":
			return countValueChars(typed["name"]) + countValueChars(typed["input"])
		case "tool_result":
			return countValueChars(typed["content"])
		case "thinking":
			return countValueChars(typed["thinking"])
		default:
			return countValueChars(typed)
		}
	default:
		return countValueChars(typed)
	}
}

func estimateAnthropicInputTokens(body map[string]any) int {
	total := countValueChars(body["system"]) + countValueChars(body["tools"])
	messages, _ := body["messages"].([]any)
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch content := message["content"].(type) {
		case string:
			total += len([]rune(content))
		case []any:
			for _, block := range content {
				total += countContentBlockChars(block)
			}
		default:
			total += countValueChars(content)
		}
	}
	return int(math.Ceil(float64(total) / 4))
}

func (g *Gateway) CountTokens(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body == nil {
		nodeReply(w, 400, map[string]any{"error": "Invalid JSON body"})
		return
	}
	nodeReply(w, 200, map[string]any{"input_tokens": estimateAnthropicInputTokens(body)})
}

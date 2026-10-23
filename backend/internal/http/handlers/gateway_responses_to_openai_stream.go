package handlers

import (
	"fmt"

	"github.com/google/uuid"
)

// Porta de open-sse/translator/response/openai-responses.js
// (openaiResponsesToOpenAIResponse): eventos da Responses API viram chunks
// OpenAI compatíveis (texto, reasoning summary e function calls incrementais).
func (s *streamConversion) toOpenAIFromResponses(event map[string]any) error {
	kind := ttsString(event["type"])
	if !s.started {
		s.started = true
		s.id = "chatcmpl-" + uuid.NewString()
		s.toolIDs = map[string]int{}
	}
	switch kind {
	case "response.output_text.delta":
		delta := ttsString(event["delta"])
		if delta == "" {
			return nil
		}
		return s.openAIChunk(map[string]any{"content": delta}, nil)
	case "response.reasoning_summary_text.delta":
		delta := ttsString(event["delta"])
		if delta == "" {
			return nil
		}
		return s.openAIChunk(map[string]any{"reasoning_content": delta}, nil)
	case "response.output_item.added":
		item, _ := event["item"].(map[string]any)
		if item == nil {
			return nil
		}
		itemType := ttsString(item["type"])
		if itemType != "function_call" && itemType != "custom_tool_call" {
			return nil
		}
		callID := ttsString(item["call_id"])
		if callID == "" {
			callID = "call_" + uuid.NewString()
		}
		index := s.toolCount
		s.toolCount++
		if id := ttsString(item["id"]); id != "" {
			s.toolIDs[id] = index
		}
		return s.openAIChunk(map[string]any{"tool_calls": []any{map[string]any{
			"index": index, "id": callID, "type": "function",
			"function": map[string]any{"name": ttsString(item["name"]), "arguments": ""},
		}}}, nil)
	case "response.function_call_arguments.delta", "response.custom_tool_call_input.delta":
		delta := ttsString(event["delta"])
		if delta == "" {
			return nil
		}
		index, ok := s.toolIDs[ttsString(event["item_id"])]
		if !ok {
			// Alguns backends omitem item_id nos deltas; o último call aberto é
			// o dono do argumento (a Responses API serializa um call por vez).
			if s.toolCount == 0 {
				return fmt.Errorf("Responses tool delta without function call")
			}
			index = s.toolCount - 1
		}
		return s.openAIChunk(map[string]any{"tool_calls": []any{map[string]any{
			"index": index, "function": map[string]any{"arguments": delta},
		}}}, nil)
	case "response.completed", "response.done", "response.incomplete":
		response, _ := event["response"].(map[string]any)
		finish := "stop"
		if s.toolCount > 0 {
			finish = "tool_calls"
		}
		if kind == "response.incomplete" || ttsString(response["status"]) == "incomplete" {
			finish = "length"
		}
		var usage map[string]any
		if raw, ok := response["usage"].(map[string]any); ok {
			usage = responsesUsageOpenAI(raw)
		}
		if s.id == nil {
			s.id = "chatcmpl-" + uuid.NewString()
		}
		if err := s.openAIChunkWithUsage(map[string]any{}, finish, usage); err != nil {
			return err
		}
		s.terminal = true
		return nil
	case "response.failed":
		response, _ := event["response"].(map[string]any)
		failure := response["error"]
		if failure == nil {
			failure = event["error"]
		}
		if object, ok := failure.(map[string]any); ok {
			message := ttsString(firstNonEmpty(object["message"], object["code"]))
			if message == "" {
				message = fmt.Sprint(object)
			}
			return fmt.Errorf("Provider responded with an error: %s", message)
		}
		return fmt.Errorf("Provider responded with an error")
	}
	return nil
}

// responsesUsageOpenAI espelha buildUsage() com os campos da Responses API:
// input_tokens já inclui os cache hits (input_tokens_details.cached_tokens).
func responsesUsageOpenAI(raw map[string]any) map[string]any {
	input := pxpipeNum(firstNonEmpty(raw["input_tokens"], raw["prompt_tokens"]))
	output := pxpipeNum(firstNonEmpty(raw["output_tokens"], raw["completion_tokens"]))
	usage := map[string]any{"prompt_tokens": input, "completion_tokens": output, "total_tokens": input + output}
	cached := 0.0
	if details, ok := raw["input_tokens_details"].(map[string]any); ok {
		cached = pxpipeNum(details["cached_tokens"])
	}
	if cached == 0 {
		cached = pxpipeNum(raw["cache_read_input_tokens"])
	}
	if cached > 0 {
		usage["prompt_tokens_details"] = map[string]any{"cached_tokens": cached}
	}
	if details, ok := raw["output_tokens_details"].(map[string]any); ok {
		if reasoning := pxpipeNum(details["reasoning_tokens"]); reasoning > 0 {
			usage["completion_tokens_details"] = map[string]any{"reasoning_tokens": reasoning}
		}
	}
	return usage
}

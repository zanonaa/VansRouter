package handlers

import (
	"encoding/json"
	"fmt"
	"time"
)

// geminiStreamChunks converte um evento SSE do Gemini no(s) chunk(s) OpenAI
// equivalentes. É a base para clientes Claude/Responses quando o upstream é
// Gemini (gemini-cli, vertex ou Gemini nativo).
func (s *streamConversion) geminiStreamChunks(event map[string]any) ([]map[string]any, error) {
	if event["responseId"] != nil {
		s.id = event["responseId"]
	}
	chunks := []map[string]any{}
	candidates, _ := event["candidates"].([]any)
	for _, raw := range candidates {
		candidate, _ := raw.(map[string]any)
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)
		for _, rawPart := range parts {
			part, _ := rawPart.(map[string]any)
			if text, ok := part["text"].(string); ok && text != "" {
				delta := map[string]any{"content": text}
				if part["thought"] == true {
					delta = map[string]any{"reasoning_content": text}
				}
				chunks = append(chunks, map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta}}})
				continue
			}
			if function, ok := part["functionCall"].(map[string]any); ok {
				arguments, err := json.Marshal(function["args"])
				if err != nil {
					return nil, err
				}
				chunks = append(chunks, map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"tool_calls": []any{map[string]any{
					"index": s.nextBlock,
					"id":    function["id"],
					"type":  "function",
					"function": map[string]any{
						"name":      function["name"],
						"arguments": string(arguments),
					},
				}}}}}})
				s.nextBlock++
				continue
			}
			return nil, fmt.Errorf("Unsupported Gemini stream part")
		}
		if reason := ttsString(candidate["finishReason"]); reason != "" {
			finish := "stop"
			if reason == "MAX_TOKENS" {
				finish = "length"
			}
			if s.nextBlock > 0 {
				finish = "tool_calls"
			}
			chunk := map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}}}
			if usage, ok := event["usageMetadata"].(map[string]any); ok {
				prompt := pxpipeNum(usage["promptTokenCount"])
				completion := pxpipeNum(usage["candidatesTokenCount"])
				chunk["usage"] = map[string]any{"prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": prompt + completion}
			}
			chunks = append(chunks, chunk)
			s.terminal = true
		}
	}
	return chunks, nil
}

func (s *streamConversion) toClaudeFromGemini(event map[string]any) error {
	if !s.started {
		s.started = true
		s.textBlock = -1
		s.thinkingBlock = -1
		s.serverTool = -1
		s.tools = map[int]int{}
		if s.id == nil {
			s.id = fmt.Sprintf("msg_%d", time.Now().UnixNano())
		}
		if err := s.emit("message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": s.id, "type": "message", "role": "assistant", "model": s.model, "content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": map[string]any{"input_tokens": 0, "output_tokens": 0}}}); err != nil {
			return err
		}
	}
	chunks, err := s.geminiStreamChunks(event)
	if err != nil {
		return err
	}
	for _, chunk := range chunks {
		choices, _ := chunk["choices"].([]any)
		for _, raw := range choices {
			choice, _ := raw.(map[string]any)
			delta, _ := choice["delta"].(map[string]any)
			if thinking := ttsString(delta["reasoning_content"]); thinking != "" {
				if s.textBlock >= 0 {
					if err := s.emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": s.textBlock}); err != nil {
						return err
					}
					s.textBlock = -1
				}
				if s.thinkingBlock < 0 {
					s.thinkingBlock = s.nextBlock
					s.nextBlock++
					if err := s.emit("content_block_start", map[string]any{"type": "content_block_start", "index": s.thinkingBlock, "content_block": map[string]any{"type": "thinking", "thinking": ""}}); err != nil {
						return err
					}
				}
				if err := s.emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": s.thinkingBlock, "delta": map[string]any{"type": "thinking_delta", "thinking": thinking}}); err != nil {
					return err
				}
			}
			if text, ok := delta["content"].(string); ok && text != "" {
				if s.thinkingBlock >= 0 {
					if err := s.emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": s.thinkingBlock}); err != nil {
						return err
					}
					s.thinkingBlock = -1
				}
				if s.textBlock < 0 {
					s.textBlock = s.nextBlock
					s.nextBlock++
					if err := s.emit("content_block_start", map[string]any{"type": "content_block_start", "index": s.textBlock, "content_block": map[string]any{"type": "text", "text": ""}}); err != nil {
						return err
					}
				}
				if err := s.emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": s.textBlock, "delta": map[string]any{"type": "text_delta", "text": text}}); err != nil {
					return err
				}
			}
			calls, _ := delta["tool_calls"].([]any)
			for _, rawCall := range calls {
				call, _ := rawCall.(map[string]any)
				function, _ := call["function"].(map[string]any)
				if s.textBlock >= 0 {
					if err := s.emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": s.textBlock}); err != nil {
						return err
					}
					s.textBlock = -1
				}
				block := s.nextBlock
				s.nextBlock++
				if err := s.emit("content_block_start", map[string]any{"type": "content_block_start", "index": block, "content_block": map[string]any{"type": "tool_use", "id": call["id"], "name": function["name"], "input": map[string]any{}}}); err != nil {
					return err
				}
				if args := ttsString(function["arguments"]); args != "" {
					if err := s.emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": block, "delta": map[string]any{"type": "input_json_delta", "partial_json": args}}); err != nil {
						return err
					}
				}
				if err := s.emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": block}); err != nil {
					return err
				}
			}
			if finish := ttsString(choice["finish_reason"]); finish != "" {
				if s.textBlock >= 0 {
					if err := s.emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": s.textBlock}); err != nil {
						return err
					}
					s.textBlock = -1
				}
				if s.thinkingBlock >= 0 {
					if err := s.emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": s.thinkingBlock}); err != nil {
						return err
					}
					s.thinkingBlock = -1
				}
				stop := "end_turn"
				if finish == "length" {
					stop = "max_tokens"
				}
				if finish == "tool_calls" {
					stop = "tool_use"
				}
				usage := map[string]any{"input_tokens": 0, "output_tokens": 0}
				if rawUsage, ok := chunk["usage"].(map[string]any); ok {
					usage["input_tokens"] = rawUsage["prompt_tokens"]
					usage["output_tokens"] = rawUsage["completion_tokens"]
				}
				if err := s.emit("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}, "usage": usage}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *streamConversion) toResponsesFromGemini(event map[string]any) error {
	chunks, err := s.geminiStreamChunks(event)
	if err != nil {
		return err
	}
	for _, chunk := range chunks {
		if s.id == nil {
			s.id = fmt.Sprintf("resp_%d", time.Now().UnixNano())
			chunk["id"] = s.id
		}
		if err := s.toResponses(chunk); err != nil {
			return err
		}
	}
	return nil
}

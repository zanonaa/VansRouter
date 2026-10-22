package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// streamConversion incrementally parses SSE frames; it never buffers a whole
// completion. All writes remain synchronous, propagating downstream backpressure.
type streamConversion struct {
	destination           http.ResponseWriter
	header                http.Header
	status                int
	pending               []byte
	source, target, model string
	started, stopped      bool
	terminal              bool
	id                    any
	usage                 map[string]any
	tools                 map[int]int
	nextBlock             int
	textBlock             int
	err                   error
	responses             *responsesStream
}

func (s *streamConversion) Header() http.Header {
	if s.header == nil {
		s.header = http.Header{}
	}
	return s.header
}
func (s *streamConversion) WriteHeader(status int) {
	if s.status == 0 {
		s.status = status
	}
}
func (s *streamConversion) Flush() {
	if f, ok := s.destination.(http.Flusher); ok {
		f.Flush()
	}
}
func (s *streamConversion) Write(body []byte) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	if s.status == 0 {
		s.status = 200
	}
	if s.status < 200 || s.status >= 300 {
		for k, v := range s.header {
			s.destination.Header()[k] = v
		}
		if !s.started {
			s.destination.WriteHeader(s.status)
			s.started = true
		}
		return s.destination.Write(body)
	}
	s.pending = append(s.pending, body...)
	for {
		i := bytes.Index(s.pending, []byte("\n\n"))
		j := bytes.Index(s.pending, []byte("\r\n\r\n"))
		delimiter := 2
		if j >= 0 && (i < 0 || j < i) {
			i = j
			delimiter = 4
		}
		if i < 0 {
			break
		}
		frame := append([]byte(nil), s.pending[:i]...)
		s.pending = s.pending[i+delimiter:]
		if err := s.frame(frame); err != nil {
			s.err = err
			return 0, err
		}
	}
	if len(s.pending) > 1<<20 {
		s.err = fmt.Errorf("SSE frame exceeds conversion limit")
		return 0, s.err
	}
	return len(body), nil
}
func (s *streamConversion) emit(event string, value any) error {
	s.destination.Header().Set("Content-Type", "text/event-stream")
	s.destination.Header().Set("Cache-Control", "no-cache")
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	frame := "data: " + string(encoded) + "\n\n"
	if event != "" {
		frame = "event: " + event + "\n" + frame
	}
	if _, err := s.destination.Write([]byte(frame)); err != nil {
		return err
	}
	s.Flush()
	return nil
}
func (s *streamConversion) frame(frame []byte) error {
	data := []string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(frame), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if len(data) == 0 {
		return nil
	}
	payload := strings.Join(data, "\n")
	if payload == "[DONE]" {
		s.terminal = true
		return s.stop()
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		return fmt.Errorf("Invalid provider SSE JSON: %w", err)
	}
	if event["error"] != nil {
		s.err = fmt.Errorf("Provider stream returned an error")
		return s.err
	}
	if s.source == "openai" {
		return s.toOpenAI(event)
	}
	if s.source == "claude" && s.target == "openai" {
		return s.toClaude(event)
	}
	if s.source == "openai-responses" && s.target == "openai" {
		return s.toResponses(event)
	}
	return fmt.Errorf("Unsupported streaming conversion: %s to %s", s.target, s.source)
}
func (s *streamConversion) openAIChunk(delta map[string]any, finish any) error {
	value := map[string]any{"id": s.id, "object": "chat.completion.chunk", "model": s.model, "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
	return s.emit("", value)
}
func (s *streamConversion) toOpenAI(event map[string]any) error {
	switch s.target {
	case "claude":
		switch event["type"] {
		case "message_start":
			message, _ := event["message"].(map[string]any)
			s.id = message["id"]
			s.started = true
			return s.openAIChunk(map[string]any{"role": "assistant", "content": ""}, nil)
		case "content_block_start":
			block, _ := event["content_block"].(map[string]any)
			index := int(pxpipeNum(event["index"]))
			if block["type"] == "tool_use" {
				if s.tools == nil {
					s.tools = map[int]int{}
				}
				toolIndex := len(s.tools)
				s.tools[index] = toolIndex
				return s.openAIChunk(map[string]any{"tool_calls": []any{map[string]any{"index": toolIndex, "id": block["id"], "type": "function", "function": map[string]any{"name": block["name"], "arguments": ""}}}}, nil)
			}
			if block["type"] != "text" {
				return fmt.Errorf("Unsupported Claude streaming block: %v", block["type"])
			}
		case "content_block_delta":
			delta, _ := event["delta"].(map[string]any)
			switch delta["type"] {
			case "text_delta":
				return s.openAIChunk(map[string]any{"content": delta["text"]}, nil)
			case "input_json_delta":
				index, ok := s.tools[int(pxpipeNum(event["index"]))]
				if !ok {
					return fmt.Errorf("Tool delta has no preceding tool block")
				}
				return s.openAIChunk(map[string]any{"tool_calls": []any{map[string]any{"index": index, "function": map[string]any{"arguments": delta["partial_json"]}}}}, nil)
			default:
				return fmt.Errorf("Unsupported Claude streaming delta: %v", delta["type"])
			}
		case "message_delta":
			delta, _ := event["delta"].(map[string]any)
			finish := "stop"
			switch delta["stop_reason"] {
			case "max_tokens":
				finish = "length"
			case "tool_use":
				finish = "tool_calls"
			}
			return s.openAIChunk(map[string]any{}, finish)
		case "message_stop":
			s.terminal = true
			return s.stop()
		}
	case "gemini":
		if event["responseId"] != nil {
			s.id = event["responseId"]
		}
		if !s.started {
			s.started = true
			if err := s.openAIChunk(map[string]any{"role": "assistant", "content": ""}, nil); err != nil {
				return err
			}
		}
		candidates, _ := event["candidates"].([]any)
		for _, raw := range candidates {
			candidate, _ := raw.(map[string]any)
			content, _ := candidate["content"].(map[string]any)
			parts, _ := content["parts"].([]any)
			for _, raw := range parts {
				part, _ := raw.(map[string]any)
				if text, ok := part["text"].(string); ok {
					if part["thought"] == true {
						return fmt.Errorf("Gemini reasoning stream mapping not implemented")
					}
					if err := s.openAIChunk(map[string]any{"content": text}, nil); err != nil {
						return err
					}
				} else if function, ok := part["functionCall"].(map[string]any); ok {
					arguments, _ := json.Marshal(function["args"])
					if err := s.openAIChunk(map[string]any{"tool_calls": []any{map[string]any{"index": s.nextBlock, "id": function["id"], "type": "function", "function": map[string]any{"name": function["name"], "arguments": string(arguments)}}}}, nil); err != nil {
						return err
					}
					s.nextBlock++
				} else {
					return fmt.Errorf("Unsupported Gemini stream part")
				}
			}
			if reason := ttsString(candidate["finishReason"]); reason != "" {
				s.terminal = true
				finish := "stop"
				if reason == "MAX_TOKENS" {
					finish = "length"
				}
				if err := s.openAIChunk(map[string]any{}, finish); err != nil {
					return err
				}
			}
		}
	default:
		return fmt.Errorf("Unsupported provider SSE format: %s", s.target)
	}
	return nil
}
func (s *streamConversion) toClaude(event map[string]any) error {
	if !s.started {
		s.started = true
		s.textBlock = -1
		s.tools = map[int]int{}
		s.id = event["id"]
		if err := s.emit("message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": s.id, "type": "message", "role": "assistant", "model": s.model, "content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": map[string]any{"input_tokens": 0, "output_tokens": 0}}}); err != nil {
			return err
		}
	}
	choices, _ := event["choices"].([]any)
	for _, raw := range choices {
		choice, _ := raw.(map[string]any)
		delta, _ := choice["delta"].(map[string]any)
		if text, ok := delta["content"].(string); ok && text != "" {
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
		for _, raw := range calls {
			call, _ := raw.(map[string]any)
			index := int(pxpipeNum(call["index"]))
			block, exists := s.tools[index]
			function, _ := call["function"].(map[string]any)
			if !exists {
				if call["id"] == nil || function["name"] == nil {
					return fmt.Errorf("Tool stream missing initial id/name")
				}
				block = s.nextBlock
				s.nextBlock++
				s.tools[index] = block
				if err := s.emit("content_block_start", map[string]any{"type": "content_block_start", "index": block, "content_block": map[string]any{"type": "tool_use", "id": call["id"], "name": function["name"], "input": map[string]any{}}}); err != nil {
					return err
				}
			}
			if text := ttsString(function["arguments"]); text != "" {
				if err := s.emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": block, "delta": map[string]any{"type": "input_json_delta", "partial_json": text}}); err != nil {
					return err
				}
			}
		}
		if reason := ttsString(choice["finish_reason"]); reason != "" {
			for i := 0; i < s.nextBlock; i++ {
				if err := s.emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": i}); err != nil {
					return err
				}
			}
			stop := "end_turn"
			if reason == "length" {
				stop = "max_tokens"
			}
			if reason == "tool_calls" {
				stop = "tool_use"
			}
			if err := s.emit("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}}); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *streamConversion) stop() error {
	if s.stopped {
		return nil
	}
	s.stopped = true
	if s.source == "openai-responses" {
		return s.completeResponses()
	}
	if s.source == "openai" {
		_, err := s.destination.Write([]byte("data: [DONE]\n\n"))
		s.Flush()
		return err
	}
	if s.source == "claude" {
		return s.emit("message_stop", map[string]any{"type": "message_stop"})
	}
	return nil
}
func (s *streamConversion) finish() {
	if s.status >= 400 {
		return
	}
	if s.err == nil && len(bytes.TrimSpace(s.pending)) > 0 {
		s.err = s.frame(s.pending)
		s.pending = nil
	}
	if s.err == nil && !s.terminal {
		s.err = fmt.Errorf("Provider stream closed before completion")
	}
	if s.err != nil {
		s.emit("error", map[string]any{"error": map[string]any{"message": s.err.Error(), "type": "stream_conversion_error"}})
		return
	}
	if !s.stopped {
		s.stop()
	}
}

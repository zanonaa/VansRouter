package handlers

import (
	"fmt"
	"sort"
	"time"
)

type responseItem struct {
	index                        int
	id, kind, callID, name, text string
	done                         bool
}
type responsesStream struct {
	sequence int
	created  int64
	items    []*responseItem
	tools    map[int]*responseItem
	message  *responseItem
	usage    map[string]any
	finish   string
	size     int
}

func (s *streamConversion) responseEvent(kind string, payload map[string]any) error {
	payload["type"] = kind
	payload["sequence_number"] = s.responses.sequence
	s.responses.sequence++
	return s.emit(kind, payload)
}
func (s *streamConversion) toResponses(event map[string]any) error {
	if s.responses == nil {
		s.id = event["id"]
		s.responses = &responsesStream{created: time.Now().Unix(), tools: map[int]*responseItem{}}
		response := map[string]any{"id": s.id, "object": "response", "created_at": s.responses.created, "status": "in_progress", "model": s.model, "output": []any{}}
		if err := s.responseEvent("response.created", map[string]any{"response": response}); err != nil {
			return err
		}
		if err := s.responseEvent("response.in_progress", map[string]any{"response": response}); err != nil {
			return err
		}
	}
	state := s.responses
	if usage, ok := event["usage"].(map[string]any); ok {
		state.usage = map[string]any{"input_tokens": usage["prompt_tokens"], "output_tokens": usage["completion_tokens"], "total_tokens": usage["total_tokens"]}
	}
	choices, _ := event["choices"].([]any)
	for _, raw := range choices {
		choice, _ := raw.(map[string]any)
		if pxpipeNum(choice["index"]) != 0 {
			return fmt.Errorf("Responses conversion supports one completion choice")
		}
		delta, _ := choice["delta"].(map[string]any)
		if delta["reasoning_content"] != nil || delta["reasoning"] != nil {
			return fmt.Errorf("Responses reasoning stream conversion not implemented")
		}
		if text, ok := delta["content"].(string); ok && text != "" {
			state.size += len(text)
			if state.size > 16<<20 {
				return fmt.Errorf("Responses output exceeds conversion limit")
			}
			if state.message == nil {
				item := &responseItem{index: len(state.items), id: fmt.Sprintf("msg_%v_0", s.id), kind: "message"}
				state.message = item
				state.items = append(state.items, item)
				if err := s.responseEvent("response.output_item.added", map[string]any{"output_index": item.index, "item": map[string]any{"id": item.id, "type": "message", "role": "assistant", "status": "in_progress", "content": []any{}}}); err != nil {
					return err
				}
				if err := s.responseEvent("response.content_part.added", map[string]any{"item_id": item.id, "output_index": item.index, "content_index": 0, "part": map[string]any{"type": "output_text", "text": "", "annotations": []any{}}}); err != nil {
					return err
				}
			}
			item := state.message
			item.text += text
			if err := s.responseEvent("response.output_text.delta", map[string]any{"item_id": item.id, "output_index": item.index, "content_index": 0, "delta": text}); err != nil {
				return err
			}
		}
		tools, _ := delta["tool_calls"].([]any)
		for _, raw := range tools {
			call, _ := raw.(map[string]any)
			index := int(pxpipeNum(call["index"]))
			function, _ := call["function"].(map[string]any)
			item := state.tools[index]
			if item == nil {
				callID, name := ttsString(call["id"]), ttsString(function["name"])
				if callID == "" || name == "" {
					return fmt.Errorf("Responses tool stream missing initial id/name")
				}
				item = &responseItem{index: len(state.items), id: "fc_" + callID, kind: "function_call", callID: callID, name: name}
				state.tools[index] = item
				state.items = append(state.items, item)
				if err := s.responseEvent("response.output_item.added", map[string]any{"output_index": item.index, "item": map[string]any{"id": item.id, "type": "function_call", "call_id": item.callID, "name": item.name, "arguments": "", "status": "in_progress"}}); err != nil {
					return err
				}
			}
			if text, ok := function["arguments"].(string); ok && text != "" {
				state.size += len(text)
				if state.size > 16<<20 {
					return fmt.Errorf("Responses output exceeds conversion limit")
				}
				item.text += text
				if err := s.responseEvent("response.function_call_arguments.delta", map[string]any{"item_id": item.id, "output_index": item.index, "delta": text}); err != nil {
					return err
				}
			}
		}
		if finish := ttsString(choice["finish_reason"]); finish != "" {
			state.finish = finish
			for _, item := range state.items {
				if err := s.closeResponseItem(item); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
func (s *streamConversion) responseItemValue(item *responseItem) map[string]any {
	if item.kind == "function_call" {
		return map[string]any{"id": item.id, "type": "function_call", "call_id": item.callID, "name": item.name, "arguments": item.text, "status": "completed"}
	}
	return map[string]any{"id": item.id, "type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": item.text, "annotations": []any{}}}}
}
func (s *streamConversion) closeResponseItem(item *responseItem) error {
	if item.done {
		return nil
	}
	item.done = true
	if item.kind == "message" {
		if err := s.responseEvent("response.output_text.done", map[string]any{"item_id": item.id, "output_index": item.index, "content_index": 0, "text": item.text}); err != nil {
			return err
		}
		if err := s.responseEvent("response.content_part.done", map[string]any{"item_id": item.id, "output_index": item.index, "content_index": 0, "part": map[string]any{"type": "output_text", "text": item.text, "annotations": []any{}}}); err != nil {
			return err
		}
	} else {
		if err := s.responseEvent("response.function_call_arguments.done", map[string]any{"item_id": item.id, "output_index": item.index, "arguments": item.text}); err != nil {
			return err
		}
	}
	return s.responseEvent("response.output_item.done", map[string]any{"output_index": item.index, "item": s.responseItemValue(item)})
}
func (s *streamConversion) completeResponses() error {
	if s.responses == nil {
		return fmt.Errorf("Responses stream completed without response data")
	}
	state := s.responses
	if state.finish == "" {
		return fmt.Errorf("Responses stream missing finish reason")
	}
	output := []any{}
	sort.SliceStable(state.items, func(i, j int) bool { return state.items[i].index < state.items[j].index })
	for _, item := range state.items {
		if err := s.closeResponseItem(item); err != nil {
			return err
		}
		output = append(output, s.responseItemValue(item))
	}
	status, kind := "completed", "response.completed"
	if state.finish == "length" {
		status, kind = "incomplete", "response.incomplete"
	}
	response := map[string]any{"id": s.id, "object": "response", "created_at": state.created, "status": status, "model": s.model, "output": output}
	if state.usage != nil {
		response["usage"] = state.usage
	}
	if status == "incomplete" {
		response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	return s.responseEvent(kind, map[string]any{"response": response})
}

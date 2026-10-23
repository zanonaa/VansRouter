package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

func commandCodeText(value any) string {
	switch content := value.(type) {
	case string:
		return content
	case []any:
		parts := []string{}
		for _, raw := range content {
			block, _ := raw.(map[string]any)
			if text := ttsString(block["text"]); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func commandCodeBlocks(value any) []any {
	blocks := []any{}
	switch content := value.(type) {
	case string:
		blocks = append(blocks, map[string]any{"type": "text", "text": content})
	case []any:
		for _, raw := range content {
			block, _ := raw.(map[string]any)
			switch block["type"] {
			case "text", "input_text", "output_text":
				blocks = append(blocks, map[string]any{"type": "text", "text": ttsString(block["text"])})
			case "image_url":
				url := ""
				switch image := block["image_url"].(type) {
				case string:
					url = image
				case map[string]any:
					url = ttsString(image["url"])
				}
				if comma := strings.Index(url, ","); comma >= 0 && strings.HasPrefix(url, "data:") {
					mime := strings.TrimSuffix(strings.TrimPrefix(url[:comma], "data:"), ";base64")
					blocks = append(blocks, map[string]any{"type": "image", "image": url, "mimeType": mime})
				}
			case "image":
				if source, ok := block["source"].(map[string]any); ok && source["type"] == "base64" {
					mime := firstNonEmptyString(source["media_type"], "image/png")
					blocks = append(blocks, map[string]any{"type": "image", "image": "data:" + mime + ";base64," + ttsString(source["data"]), "mimeType": mime})
				}
			}
		}
	}
	if len(blocks) == 0 {
		blocks = append(blocks, map[string]any{"type": "text", "text": ""})
	}
	return blocks
}

func commandCodeTarget(source map[string]any, model string, stream bool) (map[string]any, error) {
	messages, _ := source["messages"].([]any)
	converted := []any{}
	systems := []string{}
	for _, raw := range messages {
		message, _ := raw.(map[string]any)
		role := ttsString(message["role"])
		switch role {
		case "system", "developer":
			if text := commandCodeText(message["content"]); text != "" {
				systems = append(systems, text)
			}
		case "tool":
			text := commandCodeText(message["content"])
			converted = append(converted, map[string]any{"role": "tool", "content": []any{map[string]any{
				"type":       "tool-result",
				"toolCallId": ttsString(message["tool_call_id"]),
				"toolName":   ttsString(message["name"]),
				"output":     map[string]any{"type": "text", "value": text},
			}}})
		case "assistant":
			blocks := []any{}
			reasoning := firstNonEmptyString(message["reasoning_content"], message["reasoning"], message["thought"])
			calls, _ := message["tool_calls"].([]any)
			if reasoning != "" || len(calls) > 0 {
				if reasoning == "" {
					reasoning = " "
				}
				blocks = append(blocks, map[string]any{"type": "reasoning", "text": reasoning})
			}
			if text := commandCodeText(message["content"]); text != "" {
				blocks = append(blocks, map[string]any{"type": "text", "text": text})
			}
			for _, rawCall := range calls {
				call, _ := rawCall.(map[string]any)
				function, _ := call["function"].(map[string]any)
				arguments := any(map[string]any{})
				if text := ttsString(function["arguments"]); text != "" {
					if json.Unmarshal([]byte(text), &arguments) != nil {
						arguments = map[string]any{}
					}
				}
				blocks = append(blocks, map[string]any{
					"type":       "tool-call",
					"toolCallId": ttsString(call["id"]),
					"toolName":   ttsString(function["name"]),
					"input":      arguments,
				})
			}
			if len(blocks) == 0 {
				blocks = append(blocks, map[string]any{"type": "text", "text": ""})
			}
			converted = append(converted, map[string]any{"role": "assistant", "content": blocks})
		default:
			converted = append(converted, map[string]any{"role": "user", "content": commandCodeBlocks(message["content"])})
		}
	}
	params := map[string]any{
		"model":       model,
		"messages":    converted,
		"stream":      true,
		"max_tokens":  firstNonEmpty(source["max_tokens"], source["max_output_tokens"], 64000),
		"temperature": firstNonEmpty(source["temperature"], 0.3),
	}
	if len(systems) > 0 {
		params["system"] = strings.Join(systems, "\n\n")
	}
	if tools, ok := source["tools"].([]any); ok && len(tools) > 0 {
		convertedTools := []any{}
		for _, raw := range tools {
			tool, _ := raw.(map[string]any)
			if function, ok := tool["function"].(map[string]any); ok {
				convertedTools = append(convertedTools, map[string]any{
					"name":         function["name"],
					"description":  function["description"],
					"input_schema": firstNonEmpty(function["parameters"], map[string]any{"type": "object"}),
				})
			}
		}
		if len(convertedTools) > 0 {
			params["tools"] = convertedTools
		}
	}
	if value, ok := source["top_p"]; ok && value != nil {
		params["top_p"] = value
	}
	workingDir, _ := os.Getwd()
	return map[string]any{
		"threadId": uuid.NewString(),
		"memory":   "",
		"config": map[string]any{
			"workingDir":    workingDir,
			"date":          time.Now().Format("2006-01-02"),
			"environment":   runtime.GOOS,
			"structure":     []any{},
			"isGitRepo":     false,
			"currentBranch": "",
			"mainBranch":    "",
			"gitStatus":     "",
			"recentCommits": []any{},
		},
		"params": params,
	}, nil
}

type commandCodeState struct {
	id, model     string
	created       int64
	first         bool
	toolIDs       map[string]int
	toolCount     int
	finish        string
	usage         map[string]any
	content       strings.Builder
	reasoning     strings.Builder
	tools         map[int]map[string]any
	emittedFinal  bool
	upstreamError map[string]any
}

func newCommandCodeState(model string) *commandCodeState {
	return &commandCodeState{id: "chatcmpl-" + uuid.NewString(), model: model, created: time.Now().Unix(), first: true, toolIDs: map[string]int{}, tools: map[int]map[string]any{}}
}

func (state *commandCodeState) chunk(delta map[string]any, finish any) map[string]any {
	if state.first {
		delta["role"] = "assistant"
		state.first = false
	}
	return map[string]any{
		"id":      state.id,
		"object":  "chat.completion.chunk",
		"created": state.created,
		"model":   state.model,
		"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}},
	}
}

func (state *commandCodeState) handle(event map[string]any) ([]map[string]any, error) {
	kind := ttsString(event["type"])
	chunks := []map[string]any{}
	switch kind {
	case "text-delta":
		text := firstNonEmptyString(event["text"], event["delta"])
		if text == "" {
			return nil, nil
		}
		state.content.WriteString(text)
		chunks = append(chunks, state.chunk(map[string]any{"content": text}, nil))
	case "reasoning-delta":
		text := ttsString(event["text"])
		if text == "" {
			return nil, nil
		}
		state.reasoning.WriteString(text)
		chunks = append(chunks, state.chunk(map[string]any{"reasoning_content": text}, nil))
	case "tool-input-start":
		id := firstNonEmptyString(event["id"], event["toolCallId"], fmt.Sprintf("call_%d", state.toolCount))
		index := state.toolCount
		state.toolCount++
		state.toolIDs[id] = index
		state.tools[index] = map[string]any{"id": id, "name": ttsString(event["toolName"]), "arguments": ""}
		chunks = append(chunks, state.chunk(map[string]any{"tool_calls": []any{map[string]any{"index": index, "id": id, "type": "function", "function": map[string]any{"name": ttsString(event["toolName"]), "arguments": ""}}}}, nil))
	case "tool-input-delta":
		id := firstNonEmptyString(event["id"], event["toolCallId"])
		index, ok := state.toolIDs[id]
		if !ok {
			return nil, nil
		}
		delta := firstNonEmptyString(event["delta"], event["inputTextDelta"])
		if item := state.tools[index]; item != nil {
			item["arguments"] = ttsString(item["arguments"]) + delta
		}
		chunks = append(chunks, state.chunk(map[string]any{"tool_calls": []any{map[string]any{"index": index, "function": map[string]any{"arguments": delta}}}}, nil))
	case "tool-call":
		id := ttsString(event["toolCallId"])
		if _, ok := state.toolIDs[id]; ok {
			return nil, nil
		}
		index := state.toolCount
		state.toolCount++
		state.toolIDs[id] = index
		arguments := event["input"]
		encoded := ""
		switch value := arguments.(type) {
		case string:
			encoded = value
		default:
			raw, _ := json.Marshal(value)
			encoded = string(raw)
		}
		state.tools[index] = map[string]any{"id": id, "name": ttsString(event["toolName"]), "arguments": encoded}
		chunks = append(chunks, state.chunk(map[string]any{"tool_calls": []any{map[string]any{"index": index, "id": id, "type": "function", "function": map[string]any{"name": ttsString(event["toolName"]), "arguments": encoded}}}}, nil))
	case "finish-step":
		state.finish = commandCodeFinish(event["finishReason"])
		if usage, ok := event["usage"].(map[string]any); ok {
			state.usage = usage
		}
	case "finish":
		if state.finish == "" {
			state.finish = commandCodeFinish(event["finishReason"])
		}
		if usage, ok := event["totalUsage"].(map[string]any); ok {
			state.usage = usage
		}
		chunk := state.chunk(map[string]any{}, state.finish)
		if state.usage != nil {
			chunk["usage"] = commandCodeUsage(state.usage)
		}
		chunks = append(chunks, chunk)
		state.emittedFinal = true
	case "error":
		state.upstreamError = event
		return nil, fmt.Errorf("CommandCode error: %v", firstNonEmpty(event["error"], event["message"]))
	}
	return chunks, nil
}

func commandCodeFinish(value any) string {
	switch ttsString(value) {
	case "length", "max_tokens":
		return "length"
	case "tool-calls", "tool_calls":
		return "tool_calls"
	default:
		return "stop"
	}
}

func commandCodeUsage(raw map[string]any) map[string]any {
	prompt := int(pxpipeNum(firstNonEmpty(raw["promptTokens"], raw["inputTokens"], raw["prompt_tokens"])))
	completion := int(pxpipeNum(firstNonEmpty(raw["completionTokens"], raw["outputTokens"], raw["completion_tokens"])))
	return map[string]any{"prompt_tokens": prompt, "completion_tokens": completion, "total_tokens": prompt + completion}
}

func (state *commandCodeState) completion() map[string]any {
	message := map[string]any{"role": "assistant", "content": state.content.String()}
	if state.reasoning.Len() > 0 {
		message["reasoning_content"] = state.reasoning.String()
	}
	calls := []any{}
	for index := 0; index < state.toolCount; index++ {
		item := state.tools[index]
		if item == nil {
			continue
		}
		calls = append(calls, map[string]any{"id": item["id"], "type": "function", "function": map[string]any{"name": item["name"], "arguments": item["arguments"]}})
	}
	if len(calls) > 0 {
		message["tool_calls"] = calls
	}
	finish := state.finish
	if finish == "" {
		finish = "stop"
	}
	if len(calls) > 0 {
		finish = "tool_calls"
	}
	usage := map[string]any{}
	if state.usage != nil {
		usage = commandCodeUsage(state.usage)
	}
	return map[string]any{
		"id":      state.id,
		"object":  "chat.completion",
		"created": state.created,
		"model":   state.model,
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}},
		"usage":   usage,
	}
}

// SendCommandCode porta o executor/translator do CommandCode: upstream
// /alpha/generate emite NDJSON AI SDK v5; convertemos para OpenAI e depois
// para o formato do cliente.
func (t *TranslatorSend) SendCommandCode(w http.ResponseWriter, r *http.Request, provider, model string, body map[string]any, source string, stream bool) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	connections, err := t.Connections.List(r.Context(), provider, nil)
	if err != nil {
		fail(500, err.Error())
		return
	}
	var connection map[string]any
	for _, candidate := range connections {
		if candidate["isActive"] != false {
			connection = candidate
			break
		}
	}
	if connection == nil {
		fail(400, "No active connection for provider: "+provider)
		return
	}
	usageSelected(r.Context(), connection)
	intermediate := body
	if source != "openai" {
		intermediate, err = translateSource(body, source)
		if err != nil {
			fail(400, err.Error())
			return
		}
	}
	targetBody, err := commandCodeTarget(intermediate, model, true)
	if err != nil {
		fail(400, err.Error())
		return
	}
	encoded, err := json.Marshal(targetBody)
	if err != nil {
		fail(500, err.Error())
		return
	}
	target, headers, err := translatorPlan(provider, model, true, connection)
	if err != nil {
		fail(501, err.Error())
		return
	}
	headers["Accept"] = "text/event-stream"
	headers["x-session-id"] = uuid.NewString()
	request, err := http.NewRequestWithContext(r.Context(), "POST", target, bytes.NewReader(encoded))
	if err != nil {
		fail(500, err.Error())
		return
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	client, relay, cleanup, err := connectionHTTP(r.Context(), metadata, t.GetPool, t.Client)
	if err != nil {
		fail(500, err.Error())
		return
	}
	defer cleanup()
	if err := rewriteConnectionRelay(request, relay); err != nil {
		fail(500, err.Error())
		return
	}
	response, err := client.Do(request)
	if err != nil {
		fail(500, err.Error())
		return
	}
	defer response.Body.Close()
	if contentType := response.Header.Get("Content-Type"); response.StatusCode >= 200 && response.StatusCode < 300 && isHTMLContentType(contentType) {
		nodeReply(w, 502, map[string]any{"success": false, "error": fmt.Sprintf("Provider %s returned HTML instead of a chat response (content-type: %s)", provider, contentType)})
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		details, _ := io.ReadAll(response.Body)
		nodeReply(w, response.StatusCode, map[string]any{"success": false, "error": fmt.Sprintf("Provider error: %d", response.StatusCode), "details": string(details)})
		return
	}
	state := newCommandCodeState(model)
	var sink http.ResponseWriter = w
	var conversion *streamConversion
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		if source != "openai" {
			conversion = &streamConversion{destination: w, source: source, target: "openai", model: model, textBlock: -1, thinkingBlock: -1, serverTool: -1}
			sink = conversion
		}
	}
	writeChunks := func(chunks []map[string]any) {
		for _, chunk := range chunks {
			if stream {
				encoded, _ := json.Marshal(chunk)
				_, _ = fmt.Fprintf(sink, "data: %s\n\n", encoded)
				if conversion == nil {
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
				}
			}
		}
	}
	scanner := bufio.NewScanner(response.Body)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "data:") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		}
		if line == "[DONE]" {
			break
		}
		var event map[string]any
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		chunks, handleErr := state.handle(event)
		if handleErr != nil {
			if stream {
				_, _ = fmt.Fprintf(sink, "data: {\"error\":{\"message\":%q,\"type\":\"upstream_error\"}}\n\n", handleErr.Error())
				if conversion != nil {
					conversion.finish()
				}
				return
			}
			fail(502, handleErr.Error())
			return
		}
		writeChunks(chunks)
	}
	if state.upstreamError != nil {
		fail(502, fmt.Sprintf("CommandCode error: %v", firstNonEmpty(state.upstreamError["error"], state.upstreamError["message"])))
		return
	}
	if stream {
		if !state.emittedFinal {
			writeChunks([]map[string]any{state.chunk(map[string]any{}, firstNonEmpty(state.finish, "stop"))})
		}
		if conversion != nil {
			_, _ = io.WriteString(sink, "data: [DONE]\n\n")
			conversion.finish()
		} else {
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		return
	}
	completion := state.completion()
	if source != "openai" {
		converted, err := convertChatResponse(completion, "openai", source, model)
		if err != nil {
			fail(502, err.Error())
			return
		}
		completion = converted
	}
	nodeReply(w, 200, completion)
}

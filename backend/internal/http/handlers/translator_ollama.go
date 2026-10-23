package handlers

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ollamaContentText achata os blocos de conteúdo do intermediário OpenAI no
// texto que /api/chat aceita. Imagens são devolvidas separadamente em base64.
func ollamaContentText(value any) (string, []string) {
	images := []string{}
	switch content := value.(type) {
	case string:
		return content, images
	case []any:
		parts := []string{}
		for _, raw := range content {
			block, _ := raw.(map[string]any)
			switch block["type"] {
			case "text", "input_text", "output_text":
				if text := ttsString(block["text"]); text != "" {
					parts = append(parts, text)
				}
			case "image_url":
				url := ""
				switch image := block["image_url"].(type) {
				case string:
					url = image
				case map[string]any:
					url = ttsString(image["url"])
				}
				if comma := strings.Index(url, ","); comma >= 0 && strings.Contains(url[:comma], ";base64") {
					images = append(images, url[comma+1:])
				}
			}
		}
		return strings.Join(parts, "\n"), images
	}
	return "", images
}

// ollamaTarget porta openai-to-ollama.js: /api/chat só aceita content string,
// imagens em message.images[] sem o prefixo data: e mensagens de tool com
// tool_name em vez de tool_call_id.
func ollamaTarget(source map[string]any, model string, stream bool) (map[string]any, error) {
	messages, ok := source["messages"].([]any)
	if !ok {
		return nil, fmt.Errorf("messages must be an array")
	}
	toolNames := map[string]string{}
	for _, raw := range messages {
		message, _ := raw.(map[string]any)
		toolCalls, _ := message["tool_calls"].([]any)
		for _, rawCall := range toolCalls {
			call, _ := rawCall.(map[string]any)
			function, _ := call["function"].(map[string]any)
			if id, name := ttsString(call["id"]), ttsString(function["name"]); id != "" && name != "" {
				toolNames[id] = name
			}
		}
	}
	converted := []any{}
	for _, raw := range messages {
		message, _ := raw.(map[string]any)
		role := ttsString(message["role"])
		text, images := ollamaContentText(message["content"])
		if role == "tool" {
			if text == "" {
				continue
			}
			name := toolNames[ttsString(message["tool_call_id"])]
			if name == "" {
				name = ttsString(message["name"])
			}
			if name == "" {
				name = "unknown_tool"
			}
			converted = append(converted, map[string]any{"role": "tool", "tool_name": name, "content": text})
			continue
		}
		if role == "assistant" {
			if calls, ok := message["tool_calls"].([]any); ok && len(calls) > 0 {
				ollamaCalls := []any{}
				for index, rawCall := range calls {
					call, _ := rawCall.(map[string]any)
					function, _ := call["function"].(map[string]any)
					arguments := function["arguments"]
					if encoded, ok := arguments.(string); ok {
						var parsed any = map[string]any{}
						if json.Unmarshal([]byte(encoded), &parsed) != nil {
							parsed = map[string]any{}
						}
						arguments = parsed
					}
					if arguments == nil {
						arguments = map[string]any{}
					}
					ollamaCalls = append(ollamaCalls, map[string]any{
						"type": "function",
						"function": map[string]any{
							"index":     index,
							"name":      ttsString(function["name"]),
							"arguments": arguments,
						},
					})
				}
				converted = append(converted, map[string]any{"role": "assistant", "content": text, "tool_calls": ollamaCalls})
				continue
			}
		}
		if text == "" && role != "assistant" {
			continue
		}
		entry := map[string]any{"role": role, "content": text}
		if len(images) > 0 {
			entry["images"] = images
		}
		converted = append(converted, entry)
	}
	result := map[string]any{"model": model, "messages": converted, "stream": stream}
	options := map[string]any{}
	if value, ok := source["temperature"]; ok && value != nil {
		options["temperature"] = value
	}
	if value := firstNonEmpty(source["max_tokens"], source["max_completion_tokens"]); value != nil {
		options["num_predict"] = value
	}
	if value, ok := source["top_p"]; ok && value != nil {
		options["top_p"] = value
	}
	if len(options) > 0 {
		result["options"] = options
	}
	if tools, ok := source["tools"].([]any); ok && len(tools) > 0 {
		result["tools"] = tools
	}
	if choice, ok := source["tool_choice"]; ok && choice != nil {
		result["tool_choice"] = choice
	}
	return result, nil
}

func ollamaUsage(body map[string]any) map[string]any {
	prompt := int(pxpipeNum(body["prompt_eval_count"]))
	completion := int(pxpipeNum(body["eval_count"]))
	if prompt == 0 && completion == 0 {
		return nil
	}
	return map[string]any{
		"prompt_tokens":     prompt,
		"completion_tokens": completion,
		"total_tokens":      prompt + completion,
	}
}

func ollamaToolCalls(value any) []any {
	rawCalls, _ := value.([]any)
	calls := []any{}
	for index, raw := range rawCalls {
		call, _ := raw.(map[string]any)
		function, _ := call["function"].(map[string]any)
		arguments := function["arguments"]
		if _, ok := arguments.(string); !ok {
			encoded, _ := json.Marshal(arguments)
			arguments = string(encoded)
		}
		id := ttsString(call["id"])
		if id == "" {
			id = fmt.Sprintf("call_%d", index)
		}
		calls = append(calls, map[string]any{
			"index": index,
			"id":    id,
			"type":  "function",
			"function": map[string]any{
				"name":      ttsString(function["name"]),
				"arguments": arguments,
			},
		})
	}
	return calls
}

func ollamaFinish(body map[string]any, hadTools bool) string {
	if hadTools {
		return "tool_calls"
	}
	switch ttsString(body["done_reason"]) {
	case "length":
		return "length"
	case "tool_calls":
		return "tool_calls"
	default:
		return "stop"
	}
}

func ollamaChunk(id, model string, delta map[string]any, finish any, usage map[string]any) map[string]any {
	value := map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}},
	}
	if usage != nil {
		value["usage"] = usage
	}
	return value
}

func ollamaCompletion(body map[string]any, model string) map[string]any {
	message, _ := body["message"].(map[string]any)
	content := ttsString(message["content"])
	thinking := ttsString(message["thinking"])
	toolCalls := ollamaToolCalls(message["tool_calls"])
	resultMessage := map[string]any{"role": "assistant"}
	if content != "" || len(toolCalls) == 0 {
		resultMessage["content"] = content
	}
	if thinking != "" {
		resultMessage["reasoning_content"] = thinking
	}
	if len(toolCalls) > 0 {
		resultMessage["tool_calls"] = toolCalls
	}
	return map[string]any{
		"id":      fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano()),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   firstNonEmptyString(ttsString(body["model"]), model),
		"choices": []any{map[string]any{"index": 0, "message": resultMessage, "finish_reason": ollamaFinish(body, len(toolCalls) > 0)}},
		"usage":   ollamaUsage(body),
	}
}

// SendOllama executa /api/chat (NDJSON) e converte para SSE/JSON OpenAI.
// O gateway decide o target pelo provider; aqui o corpo já foi traduzido.
func (t *TranslatorSend) SendOllama(w http.ResponseWriter, r *http.Request, provider, model string, body map[string]any, source string, stream bool) {
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
	intermediate := body
	if source != "openai" {
		intermediate, err = translateSource(body, source)
		if err != nil {
			fail(400, err.Error())
			return
		}
	}
	targetBody, err := ollamaTarget(intermediate, model, stream)
	if err != nil {
		fail(400, err.Error())
		return
	}
	encoded, err := json.Marshal(targetBody)
	if err != nil {
		fail(500, err.Error())
		return
	}
	target, headers, err := translatorPlan(provider, model, stream, connection)
	if err != nil {
		fail(501, err.Error())
		return
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	request, err := http.NewRequestWithContext(r.Context(), "POST", target, bytes.NewReader(encoded))
	if err != nil {
		fail(500, err.Error())
		return
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
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
	if (response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden) && ttsString(connection["accessToken"]) != "" {
		patch, refreshErr := t.refreshConnection(r.Context(), provider, connection)
		if refreshErr == nil && patch != nil {
			retry := request.Clone(r.Context())
			retry.Body = io.NopCloser(bytes.NewReader(encoded))
			retry.Header.Set("Authorization", "Bearer "+ttsString(patch["accessToken"]))
			if second, retryErr := client.Do(retry); retryErr == nil {
				response.Body.Close()
				response = second
			}
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		details, _ := io.ReadAll(response.Body)
		nodeReply(w, response.StatusCode, map[string]any{"success": false, "error": fmt.Sprintf("Provider error: %d", response.StatusCode), "details": string(details)})
		return
	}
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		// Para clientes Claude/Responses, os chunks OpenAI são convertidos pelo
		// mesmo streamConversion usado no gateway.
		var sink http.ResponseWriter = w
		var conversion *streamConversion
		if source != "openai" {
			conversion = &streamConversion{destination: w, source: source, target: "openai", model: model, textBlock: -1, thinkingBlock: -1, serverTool: -1}
			sink = conversion
		}
		write := func(value any) {
			encoded, _ := json.Marshal(value)
			_, _ = fmt.Fprintf(sink, "data: %s\n\n", encoded)
			if conversion == nil {
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}
		}
		scanner := bufio.NewScanner(response.Body)
		scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
		id := fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
		hadTools := false
		first := true
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var event map[string]any
			if json.Unmarshal([]byte(line), &event) != nil {
				continue
			}
			message, _ := event["message"].(map[string]any)
			if event["done"] == true {
				// Alguns backends mandam o último texto junto com done=true;
				// emite antes do chunk de finish para não perder conteúdo.
				finalDelta := map[string]any{}
				if content := ttsString(message["content"]); content != "" {
					finalDelta["content"] = content
				}
				if thinking := ttsString(message["thinking"]); thinking != "" {
					finalDelta["reasoning_content"] = thinking
				}
				if calls := ollamaToolCalls(message["tool_calls"]); len(calls) > 0 {
					finalDelta["tool_calls"] = calls
					hadTools = true
				}
				if len(finalDelta) > 0 {
					if first {
						finalDelta["role"] = "assistant"
						first = false
					}
					write(ollamaChunk(id, model, finalDelta, nil, nil))
				}
				write(ollamaChunk(id, model, map[string]any{}, ollamaFinish(event, hadTools), ollamaUsage(event)))
				_, _ = io.WriteString(sink, "data: [DONE]\n\n")
				if conversion != nil {
					conversion.finish()
				}
				return
			}
			delta := map[string]any{}
			if content := ttsString(message["content"]); content != "" {
				delta["content"] = content
			}
			if thinking := ttsString(message["thinking"]); thinking != "" {
				delta["reasoning_content"] = thinking
			}
			if calls := ollamaToolCalls(message["tool_calls"]); len(calls) > 0 {
				delta["tool_calls"] = calls
				hadTools = true
			}
			if len(delta) == 0 {
				continue
			}
			if first {
				delta["role"] = "assistant"
				first = false
			}
			write(ollamaChunk(id, model, delta, nil, nil))
		}
		if conversion != nil {
			conversion.finish()
		}
		return
	}
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		fail(502, err.Error())
		return
	}
	// Ollama com stream:false devolve um único objeto; aceita NDJSON por
	// robustez caso o upstream ignore o campo.
	line := strings.TrimSpace(string(raw))
	if index := strings.IndexByte(line, '\n'); index >= 0 {
		line = strings.TrimSpace(line[:index])
	}
	var result map[string]any
	if json.Unmarshal([]byte(line), &result) != nil {
		fail(502, "Invalid Ollama response")
		return
	}
	completion := ollamaCompletion(result, model)
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

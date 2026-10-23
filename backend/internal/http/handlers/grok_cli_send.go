package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// Executor do grok-cli (porta de open-sse/executors/grok-cli.js): Responses API
// em cli-chat-proxy.grok.com, stream forçado, store=false, reasoning com
// encrypted_content para continuidade multi-turno e fingerprint de sessão nos
// headers do CLI oficial.
var (
	grokServerItemID   = regexp.MustCompile(`^(rs|fc|resp|msg)_`)
	grokNativeItemID   = regexp.MustCompile(`(?i)^(?:rs|msg|fc)_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	grokEffortSuffix   = regexp.MustCompile(`-(low|medium|high|xhigh)$`)
	grokEffortModel    = regexp.MustCompile(`^grok-4\.5(?:$|-)`)
	grokHostedToolType = map[string]bool{
		"web_search": true, "x_search": true, "web_search_preview": true,
		"file_search": true, "image_generation": true, "code_interpreter": true,
		"mcp": true, "local_shell": true,
	}
	grokResponsesFields = map[string]bool{
		"model": true, "input": true, "instructions": true, "tools": true,
		"tool_choice": true, "stream": true, "store": true, "reasoning": true,
		"include": true, "temperature": true, "top_p": true, "max_output_tokens": true,
		"parallel_tool_calls": true, "text": true, "metadata": true, "prompt_cache_key": true,
	}
)

const (
	grokTurnTTL = 2 * time.Hour
	grokTurnMax = 5000
)

type grokTurnEntry struct {
	turn     int
	lastUsed time.Time
}

var grokTurns = struct {
	sync.Mutex
	entries map[string]grokTurnEntry
}{entries: map[string]grokTurnEntry{}}

func grokNormalizeEffort(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "max":
		return "xhigh"
	case "low", "medium", "high", "xhigh":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "high"
	}
}

func grokCountUserTurns(input any) int {
	items, ok := input.([]any)
	if !ok {
		return 1
	}
	count := 0
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind := ttsString(item["type"])
		if item["role"] == "user" && (kind == "" || kind == "message") {
			count++
		}
	}
	if count < 1 {
		count = 1
	}
	return count
}

// grokResolveTurnIdx mantém o índice monotônico por sessão (a origem usa TTL de
// 2h): clientes delta enviam só o último turno, mas o header não pode regredir.
func grokResolveTurnIdx(session string, input any) int {
	fromInput := grokCountUserTurns(input)
	if session == "" {
		return fromInput
	}
	now := time.Now()
	grokTurns.Lock()
	defer grokTurns.Unlock()
	previous := 0
	if entry, ok := grokTurns.entries[session]; ok {
		if now.Sub(entry.lastUsed) <= grokTurnTTL {
			previous = entry.turn
		}
		delete(grokTurns.entries, session)
	}
	turn := fromInput
	if previous > 0 {
		turn = max(fromInput, previous+1)
	}
	for len(grokTurns.entries) >= grokTurnMax {
		for key := range grokTurns.entries {
			delete(grokTurns.entries, key)
			break
		}
	}
	grokTurns.entries[session] = grokTurnEntry{turn: turn, lastUsed: now}
	return turn
}

func grokStringifyOutput(value any, present bool) string {
	if !present {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	if value == nil {
		return "null"
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func grokNormalizeInputItem(item any) (any, bool) {
	object, ok := item.(map[string]any)
	if !ok || object == nil {
		return item, true
	}
	clean := map[string]any{}
	for key, value := range object {
		if key == "internal_chat_message_metadata_passthrough" {
			continue
		}
		clean[key] = value
	}
	switch ttsString(object["type"]) {
	case "reasoning":
		if !grokNativeItemID.MatchString(ttsString(object["id"])) {
			return nil, false
		}
		if _, ok := object["encrypted_content"].(string); !ok {
			return nil, false
		}
		return clean, true
	case "custom_tool_call":
		callID := firstNonEmptyString(object["call_id"], object["id"])
		name := strings.TrimSpace(ttsString(object["name"]))
		if callID == "" || name == "" {
			return nil, false
		}
		input, present := object["input"]
		if !present {
			input, present = object["arguments"]
		}
		arguments, _ := json.Marshal(map[string]any{"input": grokStringifyOutput(input, present)})
		return map[string]any{"type": "function_call", "call_id": callID, "name": name, "arguments": string(arguments)}, true
	case "custom_tool_call_output", "function_call_output":
		callID := firstNonEmptyString(object["call_id"], object["id"])
		if callID == "" {
			return nil, false
		}
		output, present := object["output"]
		return map[string]any{"type": "function_call_output", "call_id": callID, "output": grokStringifyOutput(output, present)}, true
	case "function_call":
		callID := firstNonEmptyString(object["call_id"], object["id"])
		name := strings.TrimSpace(ttsString(object["name"]))
		if callID == "" || name == "" {
			return nil, false
		}
		arguments := "{}"
		if text, ok := object["arguments"].(string); ok {
			arguments = text
		} else if value, present := object["arguments"]; present {
			encoded, _ := json.Marshal(value)
			arguments = string(encoded)
		}
		result := map[string]any{"type": "function_call", "call_id": callID, "name": name, "arguments": arguments}
		if grokNativeItemID.MatchString(ttsString(object["id"])) {
			result["id"] = object["id"]
		}
		if status, ok := object["status"].(string); ok {
			result["status"] = status
		}
		return result, true
	}
	return clean, true
}

func grokNormalizeInput(body map[string]any) {
	input, ok := body["input"].([]any)
	if !ok {
		return
	}
	normalized := []any{}
	for _, item := range input {
		if value, keep := grokNormalizeInputItem(item); keep {
			normalized = append(normalized, value)
		}
	}
	callIDs := map[string]bool{}
	for _, item := range normalized {
		object, _ := item.(map[string]any)
		if object == nil || object["type"] != "function_call" {
			continue
		}
		if callID := ttsString(object["call_id"]); callID != "" {
			callIDs[callID] = true
		}
	}
	filtered := []any{}
	for _, item := range normalized {
		object, _ := item.(map[string]any)
		if object != nil && object["type"] == "function_call_output" && !callIDs[ttsString(object["call_id"])] {
			continue
		}
		filtered = append(filtered, item)
	}
	body["input"] = filtered
}

// grokStripStoredItems descarta referências a itens que o upstream não resolve
// com store=false (ids de servidor que não são nativos do Grok).
func grokStripStoredItems(body map[string]any) {
	input, ok := body["input"].([]any)
	if !ok {
		return
	}
	filtered := []any{}
	for _, item := range input {
		if text, ok := item.(string); ok && grokServerItemID.MatchString(text) {
			continue
		}
		if object, ok := item.(map[string]any); ok {
			if object["type"] == "item_reference" {
				continue
			}
			if id := ttsString(object["id"]); id != "" && grokServerItemID.MatchString(id) && !grokNativeItemID.MatchString(id) {
				delete(object, "id")
			}
		}
		filtered = append(filtered, item)
	}
	body["input"] = filtered
}

func grokFreeformParameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"input": map[string]any{"type": "string"}},
		"required":   []any{"input"},
	}
}

// grokNormalizeTools achata o shape Chat Completions para o shape flat da
// Responses, preservando hosted tools (web_search/x_search) e convertendo
// tools freeform ("custom") no contrato de input único do CLI.
func grokNormalizeTools(body map[string]any) {
	rawTools, _ := body["tools"].([]any)
	if len(rawTools) == 0 {
		delete(body, "tools")
		delete(body, "tool_choice")
		return
	}
	validNames := map[string]bool{}
	hostedTypes := map[string]bool{}
	converted := []any{}
	for _, raw := range rawTools {
		tool, ok := raw.(map[string]any)
		if !ok || tool == nil {
			continue
		}
		toolType := ttsString(tool["type"])
		if toolType != "function" {
			if grokHostedToolType[toolType] {
				hostedTypes[toolType] = true
				converted = append(converted, tool)
				continue
			}
			nested, _ := tool["function"].(map[string]any)
			bareName := ttsString(tool["name"])
			if toolType != "" && nested == nil && bareName == "" {
				continue
			}
		}
		function, _ := tool["function"].(map[string]any)
		if toolType != "function" && toolType != "" && function == nil && ttsString(tool["name"]) == "" {
			continue
		}
		name := strings.TrimSpace(firstNonEmptyString(tool["name"], function["name"]))
		if name == "" {
			continue
		}
		description := firstNonEmptyString(tool["description"], function["description"])
		var parameters any
		if toolType == "custom" {
			parameters = grokFreeformParameters()
		} else if value, ok := tool["parameters"].(map[string]any); ok {
			parameters = value
		} else if value, ok := function["parameters"].(map[string]any); ok {
			parameters = value
		} else {
			parameters = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		if len(name) > 128 {
			name = name[:128]
		}
		item := map[string]any{"type": "function", "name": name, "parameters": parameters}
		if description != "" {
			item["description"] = description
		}
		validNames[name] = true
		converted = append(converted, item)
	}
	if len(converted) == 0 {
		delete(body, "tools")
		delete(body, "tool_choice")
		return
	}
	body["tools"] = converted
	if choice, ok := body["tool_choice"].(map[string]any); ok {
		choiceType := ttsString(choice["type"])
		if choiceType == "function" || choiceType == "custom" {
			rawName := choice["name"]
			if rawName == nil {
				if function, ok := choice["function"].(map[string]any); ok {
					rawName = function["name"]
				}
			}
			name := strings.TrimSpace(ttsString(rawName))
			if len(name) > 128 {
				name = name[:128]
			}
			if name == "" || !validNames[name] {
				delete(body, "tool_choice")
			} else {
				body["tool_choice"] = map[string]any{"type": "function", "name": name}
			}
		} else if !hostedTypes[choiceType] {
			delete(body, "tool_choice")
		}
	}
}

func grokResponsesInput(input any) []any {
	placeholder := func(text string) []any {
		if text == "" {
			text = "..."
		}
		return []any{map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": text}}}}
	}
	switch value := input.(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return placeholder("")
		}
		return placeholder(value)
	case []any:
		if len(value) == 0 {
			return placeholder("")
		}
		return value
	default:
		return nil
	}
}

func grokInputFromMessages(messages []any) []any {
	result := []any{}
	for _, raw := range messages {
		message, _ := raw.(map[string]any)
		if message == nil {
			continue
		}
		role := ttsString(message["role"])
		if role == "" {
			role = "user"
		}
		content := ""
		if text, ok := message["content"].(string); ok {
			content = text
		} else if message["content"] != nil {
			content = responsesText(message["content"])
		}
		result = append(result, map[string]any{"type": "message", "role": role, "content": content})
	}
	return result
}

func grokSessionID(connection, body map[string]any) string {
	for _, key := range []string{"prompt_cache_key", "session_id", "conversation_id"} {
		if value := ttsString(body[key]); value != "" {
			return value
		}
	}
	if metadata, ok := body["metadata"].(map[string]any); ok {
		for _, key := range []string{"sessionId", "session_id", "conversationId", "conversation_id"} {
			if value := ttsString(metadata[key]); value != "" {
				return value
			}
		}
	}
	if id := ttsString(connection["id"]); id != "" {
		return id
	}
	return uuid.NewString()
}

// prepareGrokCLIBody porta transformRequest(): input normalizado, tools
// achatadas, stream/store fixos, esforço de reasoning e allowlist de campos.
func prepareGrokCLIBody(body map[string]any, model string) (map[string]any, string, error) {
	prepared := map[string]any{}
	for key, value := range body {
		prepared[key] = value
	}
	if input := grokResponsesInput(prepared["input"]); input != nil {
		prepared["input"] = input
	} else if messages, ok := prepared["messages"].([]any); ok && len(messages) > 0 {
		prepared["input"] = grokInputFromMessages(messages)
		delete(prepared, "messages")
	} else {
		prepared["input"] = []any{map[string]any{"type": "message", "role": "user", "content": "..."}}
	}
	grokNormalizeInput(prepared)
	grokStripStoredItems(prepared)
	grokNormalizeTools(prepared)
	prepared["stream"] = true
	prepared["store"] = false

	modelEffort, resolvedModel := "", model
	if match := grokEffortSuffix.FindStringSubmatch(resolvedModel); match != nil {
		modelEffort = match[1]
		resolvedModel = strings.TrimSuffix(resolvedModel, match[0])
	}
	if upstream := providers.UpstreamModelID("grok-cli", resolvedModel); upstream != "" && upstream != resolvedModel {
		resolvedModel = upstream
	} else if upstream := providers.UpstreamModelID("gcli", resolvedModel); upstream != "" && upstream != resolvedModel {
		resolvedModel = upstream
	}
	prepared["model"] = resolvedModel

	supportsEffort := grokEffortModel.MatchString(resolvedModel)
	reasoning, _ := prepared["reasoning"].(map[string]any)
	if reasoning == nil {
		reasoning = map[string]any{"summary": "concise"}
		if supportsEffort {
			reasoning["effort"] = grokNormalizeEffort(firstNonEmptyString(prepared["reasoning_effort"], modelEffort))
		}
	} else {
		if supportsEffort {
			reasoning["effort"] = grokNormalizeEffort(firstNonEmptyString(reasoning["effort"], prepared["reasoning_effort"], modelEffort))
		} else {
			delete(reasoning, "effort")
		}
		if !nodeTruthy(reasoning["summary"]) {
			reasoning["summary"] = "concise"
		}
	}
	prepared["reasoning"] = reasoning
	delete(prepared, "reasoning_effort")
	if ttsString(reasoning["effort"]) != "none" {
		include, _ := prepared["include"].([]any)
		present := false
		for _, raw := range include {
			if ttsString(raw) == "reasoning.encrypted_content" {
				present = true
			}
		}
		if !present {
			include = append(include, "reasoning.encrypted_content")
		}
		prepared["include"] = include
	}

	for _, key := range []string{"messages", "max_tokens", "max_completion_tokens", "n", "seed", "logprobs", "top_logprobs", "frequency_penalty", "presence_penalty", "logit_bias", "user", "stream_options", "prompt_cache_retention", "safety_identifier", "previous_response_id"} {
		delete(prepared, key)
	}
	for key := range prepared {
		if !grokResponsesFields[key] {
			delete(prepared, key)
		}
	}
	return prepared, resolvedModel, nil
}

func grokCLIPlan(provider, model string, connection, body map[string]any) (string, map[string]string, error) {
	target, headers, err := translatorPlan(provider, model, true, connection)
	if err != nil {
		return "", nil, err
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	session := grokSessionID(connection, body)
	headers["Accept"] = "text/event-stream"
	headers["x-grok-session-id"] = session
	headers["x-grok-conv-id"] = session
	headers["x-grok-req-id"] = uuid.NewString()
	headers["x-grok-turn-idx"] = strconv.Itoa(grokResolveTurnIdx(session, body["input"]))
	headers["x-grok-model-override"] = model
	if agent := firstNonEmptyString(metadata["deviceId"], metadata["agentId"]); agent != "" {
		headers["x-grok-agent-id"] = agent
	}
	if email := firstNonEmptyString(metadata["email"], connection["email"]); email != "" {
		headers["x-email"] = email
	}
	if userID := firstNonEmptyString(metadata["userId"], metadata["principalId"], connection["userId"], connection["providerUserId"]); userID != "" {
		headers["x-userid"] = userID
	}
	return target, headers, nil
}

// SendGrokCLI executa o transporte Responses do grok-cli. Diferente do Send
// genérico, ele força stream upstream e dobra a resposta para JSON quando o
// cliente pediu non-stream (o proxy do Grok recusa stream=false).
func (t *TranslatorSend) SendGrokCLI(w http.ResponseWriter, r *http.Request, provider, model string, body map[string]any, source string, stream bool) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	if source != "openai" {
		fail(501, "Streaming conversion not implemented for format pair: "+source+" to openai-responses")
		return
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
	intermediate, err := translateSource(body, source)
	if err != nil {
		fail(501, err.Error())
		return
	}
	targetBody, err := openAIResponsesTarget(intermediate, model)
	if err != nil {
		fail(400, err.Error())
		return
	}
	prepared, resolvedModel, err := prepareGrokCLIBody(targetBody, model)
	if err != nil {
		fail(400, err.Error())
		return
	}
	target, headers, err := grokCLIPlan(provider, resolvedModel, connection, prepared)
	if err != nil {
		fail(501, err.Error())
		return
	}
	encoded, err := json.Marshal(prepared)
	if err != nil {
		fail(500, err.Error())
		return
	}
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
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		patch, refreshErr := t.refreshConnection(r.Context(), provider, connection)
		if refreshErr != nil {
			response.Body.Close()
			fail(500, refreshErr.Error())
			return
		}
		if patch != nil {
			response.Body.Close()
			retry := request.Clone(r.Context())
			retry.Body, err = request.GetBody()
			if err != nil {
				fail(500, err.Error())
				return
			}
			if ttsString(connection["apiKey"]) == "" {
				retry.Header.Set("Authorization", "Bearer "+ttsString(patch["accessToken"]))
			}
			response, err = client.Do(retry)
			if err != nil {
				fail(500, err.Error())
				return
			}
		}
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
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		conversion := &streamConversion{destination: w, source: "openai", target: "openai-responses", model: resolvedModel, textBlock: -1, serverTool: -1}
		if _, err := io.Copy(conversion, response.Body); err != nil {
			conversion.finish()
			return
		}
		conversion.finish()
		return
	}
	buffer := &bufferResponseWriter{}
	conversion := &streamConversion{destination: buffer, source: "openai", target: "openai-responses", model: resolvedModel, textBlock: -1, serverTool: -1}
	if _, err := io.Copy(conversion, response.Body); err != nil {
		conversion.finish()
	}
	conversion.finish()
	if conversion.err != nil {
		nodeReply(w, 502, map[string]any{"error": map[string]any{"message": conversion.err.Error()}})
		return
	}
	completion, err := foldOpenAIStream(buffer.body.Bytes(), resolvedModel)
	if err != nil {
		nodeReply(w, 502, map[string]any{"error": map[string]any{"message": err.Error()}})
		return
	}
	nodeReply(w, 200, completion)
}

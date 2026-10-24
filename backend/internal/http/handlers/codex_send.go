package handlers

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// Executor do Codex (porta de open-sse/executors/codex.js): Responses API em
// chatgpt.com/backend-api/codex/responses, stream forçado, store=false,
// instruções padrão embutidas, prefixo "responses lite" para os modelos GPT-6
// Sol/Luna, sessão estável de prompt cache e retry de erros SSE em corpo 200.

//go:embed codex_instructions.txt
var codexDefaultInstructions string

var (
	codexServerItemID   = regexp.MustCompile(`^(rs|fc|resp|msg)_`)
	codexLiteSuffix     = regexp.MustCompile(`\([^()]+\)\s*$`)
	codexEffortSuffix   = regexp.MustCompile(`-(none|minimal|low|medium|high|xhigh)$`)
	codexUnicodePattern = regexp.MustCompile(`(^|[^\\])(\\\\)*\\[pP]\{`)
	codexCapacityText   = regexp.MustCompile(`(?i)Selected model is at capacity\. Please try a different model\.`)
)

// Models marcados com responsesLite:true no registry da origem
// (open-sse/providers/registry/codex.js).
var codexLiteModels = map[string]bool{
	"gpt-6.1-sol":    true,
	"gpt-6-sol":      true,
	"gpt-6-sol[1m]":  true,
	"gpt-6-luna":     true,
	"gpt-6-luna[1m]": true,
}

var codexHostedToolTypes = map[string]bool{
	"image_generation": true, "web_search": true, "web_search_preview": true,
	"file_search": true, "computer": true, "computer_use_preview": true,
	"code_interpreter": true, "mcp": true, "local_shell": true, "tool_search": true,
}

var codexPassthroughToolTypes = map[string]bool{"custom": true}

var codexResponsesFields = map[string]bool{
	"model": true, "input": true, "instructions": true, "tools": true,
	"tool_choice": true, "stream": true, "store": true, "reasoning": true,
	"service_tier": true, "include": true, "prompt_cache_key": true,
	"client_metadata": true, "text": true, "parallel_tool_calls": true,
}

const (
	codexSSEPeekBytes   = 256 * 1024
	codexRetryAttempts  = 3
	codexSessionTTL     = 2 * time.Hour
	codexSessionMax     = 5000
	codexImageMaxBytes  = 10 * 1024 * 1024
	codexImageFetchTime = 15 * time.Second
)

var (
	codexSSERetryPatterns = []string{"server_is_overloaded", "service_unavailable_error"}
	codexSSEAccountHits   = []string{"selected model is at capacity", "model_at_capacity"}
	codexSSEOutputHits    = []string{
		"event: response.output_text.delta",
		"event: response.function_call_arguments.delta",
		`"type":"response.output_text.delta"`,
		`"type":"response.function_call_arguments.delta"`,
	}
	// Variáveis para os testes não esperarem os 2s da origem.
	codexRetryDelay = 2 * time.Second
)

// ---------------------------------------------------------------------------
// Sessão de prompt cache (open-sse/utils/sessionManager.js)

type codexPromptSession struct {
	sessionID string
	lastUsed  time.Time
}

var codexPromptSessions = struct {
	sync.Mutex
	byConnection map[string]codexPromptSession
	byAssistant  map[string]codexPromptSession
}{
	byConnection: map[string]codexPromptSession{},
	byAssistant:  map[string]codexPromptSession{},
}

var codexClaudeSessionRE = regexp.MustCompile(`_session_([a-f0-9-]+)$`)

func codexBinaryStyleID() string {
	return uuid.NewString() + strconv.FormatInt(time.Now().UnixMilli(), 10)
}

func codexNormalizeSession(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 256 {
		return ""
	}
	return value
}

func codexHeaderValue(headers http.Header, key string) string {
	if headers == nil {
		return ""
	}
	return codexNormalizeSession(headers.Get(key))
}

func codexClaudeSession(userID string) string {
	if userID == "" {
		return ""
	}
	if match := codexClaudeSessionRE.FindStringSubmatch(userID); match != nil {
		return match[1]
	}
	if strings.HasPrefix(userID, "{") {
		var parsed map[string]any
		if json.Unmarshal([]byte(userID), &parsed) == nil {
			return codexNormalizeSession(ttsString(parsed["session_id"]))
		}
	}
	return ""
}

func codexClientSession(headers http.Header, body map[string]any) string {
	metadata, _ := body["metadata"].(map[string]any)
	userID := ttsString(metadata["user_id"])
	if session := codexClaudeSession(userID); session != "" {
		return "claude:" + session
	}
	if session := codexHeaderValue(headers, "x-claude-code-session-id"); session != "" {
		return "claude:" + session
	}
	for _, key := range []string{"x-session-id", "session-id", "session_id", "x-amp-thread-id"} {
		if value := codexHeaderValue(headers, key); value != "" {
			return value
		}
	}
	if value := codexHeaderValue(headers, "x-client-request-id"); value != "" {
		return value
	}
	for _, key := range []string{"prompt_cache_key", "session_id", "conversation_id"} {
		if value := codexNormalizeSession(ttsString(body[key])); value != "" {
			return value
		}
	}
	if value := codexNormalizeSession(userID); value != "" {
		return value
	}
	return ""
}

func codexAssistantText(body map[string]any) string {
	items, _ := body["messages"].([]any)
	if items == nil {
		items, _ = body["input"].([]any)
	}
	text := strings.Builder{}
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		kind := ttsString(item["type"])
		if item["role"] != "assistant" && kind != "function_call" && kind != "custom_tool_call" {
			continue
		}
		switch content := item["content"].(type) {
		case string:
			text.WriteString(content)
		case []any:
			for _, rawBlock := range content {
				switch block := rawBlock.(type) {
				case string:
					text.WriteString(block)
				case map[string]any:
					text.WriteString(firstNonEmptyString(block["text"], block["output"]))
				}
			}
		}
		if calls, ok := item["tool_calls"].([]any); ok {
			for _, rawCall := range calls {
				call, _ := rawCall.(map[string]any)
				function, _ := call["function"].(map[string]any)
				text.WriteString(ttsString(function["name"]) + ":" + ttsString(function["arguments"]))
			}
		}
		if item["name"] != nil || item["arguments"] != nil {
			text.WriteString(ttsString(item["name"]) + ":" + ttsString(item["arguments"]))
		}
		if text.Len() >= 50 {
			break
		}
	}
	return text.String()
}

func codexAssistantSession(connectionID string, body map[string]any) string {
	text := codexAssistantText(body)
	if len(text) < 50 {
		return ""
	}
	hash := sha256.Sum256([]byte("codex:" + connectionID + ":" + text[:50]))
	key := hex.EncodeToString(hash[:])[:16]

	codexPromptSessions.Lock()
	defer codexPromptSessions.Unlock()
	now := time.Now()
	if entry, ok := codexPromptSessions.byAssistant[key]; ok && now.Sub(entry.lastUsed) <= codexSessionTTL {
		entry.lastUsed = now
		codexPromptSessions.byAssistant[key] = entry
		return entry.sessionID
	}
	if len(codexPromptSessions.byAssistant) >= codexSessionMax {
		for stale := range codexPromptSessions.byAssistant {
			delete(codexPromptSessions.byAssistant, stale)
			break
		}
	}
	session := codexBinaryStyleID()
	codexPromptSessions.byAssistant[key] = codexPromptSession{sessionID: session, lastUsed: now}
	return session
}

func codexConnectionSession(connectionID string) string {
	if connectionID == "" {
		return codexBinaryStyleID()
	}
	codexPromptSessions.Lock()
	defer codexPromptSessions.Unlock()
	now := time.Now()
	if entry, ok := codexPromptSessions.byConnection[connectionID]; ok && now.Sub(entry.lastUsed) <= codexSessionTTL {
		entry.lastUsed = now
		codexPromptSessions.byConnection[connectionID] = entry
		return entry.sessionID
	}
	if len(codexPromptSessions.byConnection) >= 1000 {
		for stale := range codexPromptSessions.byConnection {
			delete(codexPromptSessions.byConnection, stale)
			break
		}
	}
	session := codexBinaryStyleID()
	codexPromptSessions.byConnection[connectionID] = codexPromptSession{sessionID: session, lastUsed: now}
	return session
}

func codexSessionID(connection map[string]any, headers http.Header, body map[string]any) string {
	if session := codexClientSession(headers, body); session != "" {
		return session
	}
	connectionID := ttsString(connection["id"])
	if session := codexAssistantSession(connectionID, body); session != "" {
		return session
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	if workspace := codexNormalizeSession(ttsString(metadata["workspaceId"])); workspace != "" {
		return workspace
	}
	return codexConnectionSession(connectionID)
}

// ---------------------------------------------------------------------------
// Helpers de modelo, thinking e input

func codexIsLite(model string) bool {
	base := codexLiteSuffix.ReplaceAllString(model, "")
	return codexLiteModels[base]
}

func codexLevels(model string) []string {
	switch {
	case codexIsLite(model):
		return []string{"low", "medium", "high", "xhigh", "max"}
	case strings.Contains(model, "gpt-5.6-sol"), strings.Contains(model, "gpt-5.6-terra"),
		strings.Contains(model, "gpt-5.6-luna"), strings.Contains(model, "gpt-6"):
		return []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}
	case strings.Contains(model, "codex"):
		return []string{"low", "medium", "high", "xhigh"}
	default:
		return []string{"none", "minimal", "low", "medium", "high", "xhigh"}
	}
}

func codexLevelSupported(model, level string) bool {
	for _, candidate := range codexLevels(model) {
		if candidate == level {
			return true
		}
	}
	return false
}

func codexNormalizeEffort(model, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "ultra" && (strings.Contains(model, "gpt-5.6-sol") || strings.Contains(model, "gpt-5.6-terra")) {
		return value
	}
	if codexLevelSupported(model, value) {
		return value
	}
	if codexIsLite(model) && (value == "none" || value == "minimal") {
		return "low"
	}
	if value == "ultra" && codexLevelSupported(model, "max") {
		return "max"
	}
	if value == "max" || value == "ultra" {
		return "xhigh"
	}
	return value
}

func codexResponsesInput(input any) []any {
	placeholder := func(text string) []any {
		if text == "" {
			text = "..."
		}
		return []any{map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": text}}}}
	}
	switch value := input.(type) {
	case string:
		return placeholder(strings.TrimSpace(value))
	case []any:
		if len(value) == 0 {
			return placeholder("")
		}
		return value
	}
	return nil
}

func codexConvertSystemToDeveloper(body map[string]any) {
	input, ok := body["input"].([]any)
	if !ok {
		return
	}
	for _, raw := range input {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind := ttsString(item["type"])
		if item["role"] == "system" && (kind == "" || kind == "message") {
			item["role"] = "developer"
		}
	}
}

func codexStripStoredItems(body map[string]any, preserveLitePrefix bool) {
	input, ok := body["input"].([]any)
	if !ok {
		return
	}
	filtered := []any{}
	for _, raw := range input {
		if text, ok := raw.(string); ok && codexServerItemID.MatchString(text) {
			continue
		}
		item, _ := raw.(map[string]any)
		if item != nil {
			if item["type"] == "item_reference" {
				continue
			}
			id := ttsString(item["id"])
			preserve := preserveLitePrefix && item["role"] == "developer" && strings.HasPrefix(id, "msg_")
			if id != "" && codexServerItemID.MatchString(id) && !preserve {
				delete(item, "id")
				// A origem muta o item e o mantém na lista.
			}
		}
		filtered = append(filtered, raw)
	}
	body["input"] = filtered
}

// codexStripUnsupportedPatterns remove `pattern` com \p{...}/\P{...}, que o
// validador de schema do Codex não entende (open-sse/utils/codexToolSchema.js).
func codexStripUnsupportedPatterns(node any) any {
	switch value := node.(type) {
	case []any:
		for index, item := range value {
			value[index] = codexStripUnsupportedPatterns(item)
		}
		return value
	case map[string]any:
		for key, item := range value {
			if key == "pattern" {
				if text, ok := item.(string); ok && codexUnicodePattern.MatchString(text) {
					delete(value, key)
					continue
				}
			}
			value[key] = codexStripUnsupportedPatterns(item)
		}
		return value
	default:
		return node
	}
}

func codexToolName(tool map[string]any) string {
	if name := ttsString(tool["name"]); name != "" {
		return name
	}
	if function, ok := tool["function"].(map[string]any); ok {
		return ttsString(function["name"])
	}
	return ""
}

func codexNormalizeTools(body map[string]any) {
	rawTools, ok := body["tools"].([]any)
	if !ok {
		return
	}
	validNames := map[string]bool{}
	converted := []any{}
	for _, raw := range rawTools {
		tool, ok := raw.(map[string]any)
		if !ok || tool == nil {
			continue
		}
		toolType := ttsString(tool["type"])
		if toolType == "namespace" {
			if nested, ok := tool["tools"].([]any); ok {
				for _, rawNested := range nested {
					nestedTool, _ := rawNested.(map[string]any)
					if name := strings.TrimSpace(ttsString(nestedTool["name"])); name != "" {
						if len(name) > 128 {
							name = name[:128]
						}
						validNames[name] = true
					}
					if parameters, ok := nestedTool["parameters"].(map[string]any); ok {
						nestedTool["parameters"] = codexStripUnsupportedPatterns(parameters)
					}
				}
			}
			converted = append(converted, tool)
			continue
		}
		if toolType != "function" {
			if codexPassthroughToolTypes[toolType] {
				converted = append(converted, tool)
				continue
			}
			if toolType == "" || tool["function"] != nil || ttsString(tool["name"]) != "" {
				continue
			}
			if codexHostedToolTypes[toolType] {
				converted = append(converted, tool)
			}
			continue
		}
		function, _ := tool["function"].(map[string]any)
		rawName := ttsString(tool["name"])
		if rawName == "" {
			rawName = ttsString(function["name"])
		}
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		if len(name) > 128 {
			name = name[:128]
		}
		description := ttsString(tool["description"])
		if description == "" {
			description = ttsString(function["description"])
		}
		parameters := any(map[string]any{"type": "object", "properties": map[string]any{}})
		if value, ok := tool["parameters"].(map[string]any); ok {
			parameters = value
		} else if value, ok := function["parameters"].(map[string]any); ok {
			parameters = value
		}
		item := map[string]any{"type": "function", "name": name, "parameters": codexStripUnsupportedPatterns(parameters)}
		if description != "" {
			item["description"] = description
		}
		validNames[name] = true
		converted = append(converted, item)
	}
	body["tools"] = converted
	if choice, ok := body["tool_choice"].(map[string]any); ok && ttsString(choice["type"]) == "function" {
		name := strings.TrimSpace(ttsString(choice["name"]))
		if name == "" || !validNames[name] {
			delete(body, "tool_choice")
		}
	}
}

func codexHasTool(body map[string]any, toolType string) bool {
	tools, _ := body["tools"].([]any)
	for _, raw := range tools {
		tool, _ := raw.(map[string]any)
		if ttsString(tool["type"]) == toolType {
			return true
		}
	}
	return false
}

// codexPrefetchImages porta CodexExecutor.prefetchImages: o backend do Codex
// não baixa imagens remotas, então image_url vira input_image com data URI.
func codexPrefetchImages(ctx context.Context, body map[string]any) {
	input, _ := body["input"].([]any)
	for _, raw := range input {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		content, ok := item["content"].([]any)
		if !ok {
			continue
		}
		for index, rawBlock := range content {
			block, _ := rawBlock.(map[string]any)
			if block == nil || ttsString(block["type"]) != "image_url" {
				continue
			}
			imageURL, detail := "", "auto"
			switch image := block["image_url"].(type) {
			case string:
				imageURL = image
			case map[string]any:
				imageURL = ttsString(image["url"])
				if value := ttsString(image["detail"]); value != "" {
					detail = value
				}
			}
			if imageURL == "" {
				continue
			}
			if strings.HasPrefix(imageURL, "data:") {
				content[index] = map[string]any{"type": "input_image", "image_url": imageURL, "detail": detail}
				continue
			}
			fetched := codexFetchImageDataURI(ctx, imageURL)
			if fetched == "" {
				fetched = imageURL
			}
			content[index] = map[string]any{"type": "input_image", "image_url": fetched, "detail": detail}
		}
		item["content"] = content
	}
}

func codexPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// CGNAT 100.64.0.0/10 e metadados de nuvem 169.254.169.254.
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return true
		}
	}
	return false
}

func codexImageMime(data []byte) string {
	switch {
	case len(data) >= 4 && bytes.Equal(data[:4], []byte{0x89, 'P', 'N', 'G'}):
		return "image/png"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xff, 0xd8, 0xff}):
		return "image/jpeg"
	case len(data) >= 4 && bytes.Equal(data[:4], []byte{'G', 'I', 'F', '8'}):
		return "image/gif"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte{'R', 'I', 'F', 'F'}) && bytes.Equal(data[8:12], []byte{'W', 'E', 'B', 'P'}):
		return "image/webp"
	case len(data) >= 2 && data[0] == 'B' && data[1] == 'M':
		return "image/bmp"
	}
	return ""
}

func codexFetchImageDataURI(ctx context.Context, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	host := parsed.Hostname()
	if host == "" {
		return ""
	}
	blocked := map[string]bool{"localhost": true, "127.0.0.1": true, "0.0.0.0": true, "::1": true, "169.254.169.254": true, "metadata.google.internal": true}
	if blocked[strings.ToLower(host)] {
		return ""
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, codexImageFetchTime)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupIPAddr(timeoutCtx, host)
	if err != nil || len(addresses) == 0 {
		return ""
	}
	for _, address := range addresses {
		if codexPrivateIP(address.IP) {
			return ""
		}
	}
	pinned := addresses[0].IP
	dialer := &net.Dialer{Timeout: codexImageFetchTime}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(dialCtx context.Context, network, address string) (net.Conn, error) {
			_, port, err := net.SplitHostPort(address)
			if err != nil {
				port = "80"
				if parsed.Scheme == "https" {
					port = "443"
				}
			}
			return dialer.DialContext(dialCtx, network, net.JoinHostPort(pinned.String(), port))
		},
		TLSClientConfig:   &tls.Config{ServerName: host},
		DisableKeepAlives: true,
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{
		Transport: transport,
		Timeout:   codexImageFetchTime,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	request, err := http.NewRequestWithContext(timeoutCtx, "GET", rawURL, nil)
	if err != nil {
		return ""
	}
	response, err := client.Do(request)
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, codexImageMaxBytes+1))
	if err != nil || len(data) == 0 || len(data) > codexImageMaxBytes {
		return ""
	}
	mime := codexImageMime(data)
	if mime == "" {
		return ""
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// ---------------------------------------------------------------------------
// Transform do body

// prepareCodexBody porta CodexExecutor.transformRequest. Retorna o corpo
// final, o modelo upstream resolvido e o session_id usado nos headers.
func prepareCodexBody(body map[string]any, model string, connection map[string]any, headers http.Header) (map[string]any, string, string, error) {
	prepared := map[string]any{}
	for key, value := range body {
		prepared[key] = value
	}
	delete(prepared, "_compact")

	session := codexSessionID(connection, headers, prepared)
	if input := codexResponsesInput(prepared["input"]); input != nil {
		prepared["input"] = input
	}

	resolved := providers.UpstreamModelID("codex", ttsString(firstNonEmpty(prepared["model"], model)))

	autoWebSearch := nodeTruthy(prepared["_autoCodexWebSearch"])
	delete(prepared, "_autoCodexWebSearch")
	if autoWebSearch && !codexHasTool(prepared, "web_search") {
		tools, _ := prepared["tools"].([]any)
		prepared["tools"] = append(tools, map[string]any{"type": "web_search"})
	}

	// Modelos Lite não executam web_search dentro do prefixo de tools: migra o
	// prefixo para tools normais e cai no transporte Responses completo.
	convertedLitePrefix := false
	if codexIsLite(resolved) {
		if input, ok := prepared["input"].([]any); ok && (codexHasTool(prepared, "web_search") || codexInputHasWebSearch(input)) {
			tools, _ := prepared["tools"].([]any)
			merged := append([]any{}, tools...)
			seen := map[string]bool{}
			for _, raw := range merged {
				tool, _ := raw.(map[string]any)
				seen[ttsString(tool["type"])+":"+codexToolName(tool)] = true
			}
			for _, raw := range input {
				item, _ := raw.(map[string]any)
				if item == nil || ttsString(item["type"]) != "additional_tools" {
					continue
				}
				itemTools, _ := item["tools"].([]any)
				for _, rawTool := range itemTools {
					tool, _ := rawTool.(map[string]any)
					key := ttsString(tool["type"]) + ":" + codexToolName(tool)
					if !seen[key] {
						merged = append(merged, rawTool)
						seen[key] = true
					}
				}
			}
			prepared["tools"] = merged
			filtered := []any{}
			for _, raw := range input {
				item, _ := raw.(map[string]any)
				if item != nil && ttsString(item["type"]) == "additional_tools" {
					convertedLitePrefix = true
					continue
				}
				filtered = append(filtered, raw)
			}
			prepared["input"] = filtered
		}
	}
	responsesLite := codexIsLite(resolved) && !codexHasTool(prepared, "web_search")

	if input, ok := prepared["input"].([]any); !ok || len(input) == 0 {
		prepared["input"] = codexResponsesInput("")
	}
	codexConvertSystemToDeveloper(prepared)
	codexStripStoredItems(prepared, responsesLite)
	codexNormalizeTools(prepared)
	prepared["stream"] = true
	prepared["store"] = false
	if !responsesLite && !convertedLitePrefix && strings.TrimSpace(ttsString(prepared["instructions"])) == "" {
		prepared["instructions"] = codexDefaultInstructions
	}
	if !nodeTruthy(prepared["prompt_cache_key"]) && session != "" {
		prepared["prompt_cache_key"] = session
	}
	prepared["model"] = resolved

	if responsesLite {
		input, _ := prepared["input"].([]any)
		hasLitePrefix := false
		for _, raw := range input {
			item, _ := raw.(map[string]any)
			if item != nil && ttsString(item["type"]) == "additional_tools" {
				hasLitePrefix = true
				break
			}
		}
		if !hasLitePrefix {
			instructions := ttsString(prepared["instructions"])
			if strings.TrimSpace(instructions) == "" {
				instructions = codexDefaultInstructions
			}
			tools, _ := prepared["tools"].([]any)
			if tools == nil {
				tools = []any{}
			}
			prefix := []any{map[string]any{"type": "additional_tools", "role": "developer", "tools": tools}}
			if instructions != "" {
				prefix = append(prefix, map[string]any{
					"type": "message", "role": "developer",
					"content": []any{map[string]any{"type": "input_text", "text": instructions}},
				})
			}
			prepared["input"] = append(prefix, input...)
		}
		prepared["instructions"] = ""
		prepared["tools"] = nil
		if !nodeTruthy(prepared["tool_choice"]) {
			prepared["tool_choice"] = "auto"
		}
		prepared["parallel_tool_calls"] = false
	} else {
		delete(prepared, "parallel_tool_calls")
	}

	modelEffort := ""
	if match := codexEffortSuffix.FindStringSubmatch(resolved); match != nil {
		modelEffort = match[1]
		resolved = strings.TrimSuffix(resolved, match[0])
		prepared["model"] = resolved
	}

	defaultEffort := "low"
	if responsesLite {
		defaultEffort = "medium"
	}
	reasoning, _ := prepared["reasoning"].(map[string]any)
	if reasoning == nil {
		effort := codexNormalizeEffort(resolved, firstNonEmptyString(prepared["reasoning_effort"], modelEffort, defaultEffort))
		reasoning = map[string]any{"effort": effort}
		if !responsesLite {
			reasoning["summary"] = "auto"
		}
	} else {
		current := firstNonEmptyString(reasoning["effort"], prepared["reasoning_effort"], modelEffort, defaultEffort)
		reasoning["effort"] = codexNormalizeEffort(resolved, current)
		if !responsesLite && !nodeTruthy(reasoning["summary"]) {
			reasoning["summary"] = "auto"
		}
	}
	if responsesLite {
		reasoning["context"] = "all_turns"
	}
	prepared["reasoning"] = reasoning
	delete(prepared, "reasoning_effort")
	if effort := ttsString(reasoning["effort"]); effort != "" && effort != "none" {
		prepared["include"] = []any{"reasoning.encrypted_content"}
	}

	for _, key := range []string{
		"temperature", "top_p", "frequency_penalty", "presence_penalty", "logprobs",
		"top_logprobs", "n", "seed", "max_tokens", "max_completion_tokens",
		"max_output_tokens", "user", "prompt_cache_retention", "metadata",
		"stream_options", "safety_identifier", "previous_response_id",
	} {
		delete(prepared, key)
	}
	if tier := ttsString(prepared["service_tier"]); tier == "fast" {
		prepared["service_tier"] = "priority"
	} else if tier != "" && tier != "priority" {
		delete(prepared, "service_tier")
	}
	for key := range prepared {
		if !codexResponsesFields[key] {
			delete(prepared, key)
		}
	}
	return prepared, resolved, session, nil
}

func codexInputHasWebSearch(input []any) bool {
	for _, raw := range input {
		item, _ := raw.(map[string]any)
		if item == nil || ttsString(item["type"]) != "additional_tools" {
			continue
		}
		tools, _ := item["tools"].([]any)
		for _, rawTool := range tools {
			tool, _ := rawTool.(map[string]any)
			if tool != nil && ttsString(tool["type"]) == "web_search" {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Erros SSE em corpo 200

func codexNestedMessage(value any, depth int) string {
	if value == nil || depth > 6 {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return ""
	case []any:
		for _, item := range typed {
			if message := codexNestedMessage(item, depth+1); message != "" {
				return message
			}
		}
		return ""
	case map[string]any:
		if message := strings.TrimSpace(ttsString(typed["message"])); message != "" {
			return message
		}
		if errObject, ok := typed["error"].(map[string]any); ok {
			if message := strings.TrimSpace(ttsString(errObject["message"])); message != "" {
				return message
			}
		}
		if response, ok := typed["response"].(map[string]any); ok {
			if errObject, ok := response["error"].(map[string]any); ok {
				if message := strings.TrimSpace(ttsString(errObject["message"])); message != "" {
					return message
				}
			}
		}
		for _, child := range typed {
			if message := codexNestedMessage(child, depth+1); message != "" {
				return message
			}
		}
	}
	return ""
}

func codexSSEMessage(text, fallback string) string {
	if exact := codexCapacityText.FindString(text); exact != "" {
		return exact
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event any
		if json.Unmarshal([]byte(payload), &event) != nil {
			continue
		}
		if message := codexNestedMessage(event, 0); message != "" {
			return message
		}
	}
	if fallback != "" {
		return fallback
	}
	return "Selected model is at capacity. Please try a different model."
}

// codexPeekSSE lê até 256KB do stream para detectar erros transitórios que vêm
// com HTTP 200. Retorna os bytes já lidos para reconstruir o stream.
func codexPeekSSE(reader *bufio.Reader) (prefix []byte, matched string, accountFallback bool, message string) {
	text := strings.Builder{}
	buffer := make([]byte, 4096)
	for len(prefix) < codexSSEPeekBytes {
		count, err := reader.Read(buffer)
		if count > 0 {
			prefix = append(prefix, buffer[:count]...)
			text.Write(buffer[:count])
			lower := strings.ToLower(text.String())
			for _, pattern := range codexSSEAccountHits {
				if strings.Contains(lower, pattern) {
					return prefix, pattern, true, codexSSEMessage(text.String(), pattern)
				}
			}
			for _, pattern := range codexSSERetryPatterns {
				if strings.Contains(lower, pattern) {
					return prefix, pattern, false, codexSSEMessage(text.String(), pattern)
				}
			}
			for _, pattern := range codexSSEOutputHits {
				if strings.Contains(lower, pattern) {
					return prefix, "", false, ""
				}
			}
		}
		if err != nil {
			break
		}
	}
	return prefix, "", false, ""
}

func codexSSEErrorResponse(w http.ResponseWriter, message string) {
	nodeReply(w, http.StatusServiceUnavailable, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    "server_error",
			"code":    "service_unavailable",
		},
	})
}

// foldResponsesStream porta o fold de SSE Responses usado quando o cliente
// pediu JSON: devolve o objeto final de response.completed/incomplete.
func foldResponsesStream(raw []byte) (map[string]any, error) {
	var final map[string]any
	var failure error
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			return nil, fmt.Errorf("Invalid provider SSE JSON: %w", err)
		}
		switch ttsString(event["type"]) {
		case "response.completed", "response.incomplete", "response.done":
			if response, ok := event["response"].(map[string]any); ok {
				final = response
			}
		case "response.failed":
			response, _ := event["response"].(map[string]any)
			message := ""
			if errObject, ok := response["error"].(map[string]any); ok {
				message = firstNonEmptyString(errObject["message"], errObject["code"])
			}
			if message == "" {
				message = "Provider responded with an error"
			}
			failure = fmt.Errorf("%s", message)
		default:
			if event["error"] != nil {
				failure = fmt.Errorf("Provider stream returned an error: %s", codexNestedMessage(event["error"], 0))
			}
		}
	}
	if failure != nil {
		return nil, failure
	}
	if final == nil {
		return nil, fmt.Errorf("Provider returned an empty stream")
	}
	return final, nil
}

// ---------------------------------------------------------------------------
// Executor

// SendCodex executa o transporte Responses do Codex. O upstream sempre
// transmite SSE; clientes OpenAI recebem chunks convertidos e clientes
// /v1/responses recebem o SSE nativo (ou o JSON dobrado).
func (t *TranslatorSend) SendCodex(w http.ResponseWriter, r *http.Request, provider, model string, body map[string]any, source string, stream bool) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	if source != "openai" && source != "openai-responses" {
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
	usageSelected(r.Context(), connection)
	var intermediate map[string]any
	if source == "openai" {
		intermediate, err = translateSource(body, source)
	} else {
		intermediate = body
	}
	if err != nil {
		fail(501, err.Error())
		return
	}
	targetBody, err := openAIResponsesTarget(intermediate, model)
	if err != nil {
		fail(400, err.Error())
		return
	}
	compact := nodeTruthy(targetBody["_compact"])
	t.TokenSaver.Apply(r.Context(), r, targetBody, "openai-responses", model, provider)
	prepared, resolvedModel, session, err := prepareCodexBody(targetBody, model, connection, r.Header)
	if err != nil {
		fail(400, err.Error())
		return
	}
	// O prefetch precisa do input final (inclusive o prefixo lite).
	codexPrefetchImages(r.Context(), prepared)
	encoded, err := json.Marshal(prepared)
	if err != nil {
		fail(500, err.Error())
		return
	}
	target, headers, err := translatorPlan(provider, resolvedModel, true, connection)
	if err != nil {
		fail(501, err.Error())
		return
	}
	if compact {
		target = strings.TrimRight(target, "/") + "/compact"
	}
	headers["Accept"] = "text/event-stream"
	headers["session_id"] = session
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	accountID := firstNonEmptyString(metadata["workspaceId"], metadata["chatgptAccountId"], metadata["accountId"])
	if accountID != "" {
		headers["ChatGPT-Account-ID"] = accountID
	}
	if codexIsLite(resolvedModel) && !codexHasTool(prepared, "web_search") {
		headers["x-openai-internal-codex-responses-lite"] = "true"
	}

	newRequest := func() (*http.Request, error) {
		request, err := http.NewRequestWithContext(r.Context(), "POST", target, bytes.NewReader(encoded))
		if err != nil {
			return nil, err
		}
		for key, value := range headers {
			request.Header.Set(key, value)
		}
		return request, nil
	}
	client, relay, cleanup, err := connectionHTTP(r.Context(), metadata, t.GetPool, t.Client)
	if err != nil {
		fail(500, err.Error())
		return
	}
	defer cleanup()

	refreshTried := false
	sseAttempt := 0
	for {
		request, err := newRequest()
		if err != nil {
			fail(500, err.Error())
			return
		}
		if err := rewriteConnectionRelay(request, relay); err != nil {
			fail(500, err.Error())
			return
		}
		response, err := client.Do(request)
		if err != nil {
			fail(500, err.Error())
			return
		}
		if (response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden) && !refreshTried {
			refreshTried = true
			patch, refreshErr := t.refreshConnection(r.Context(), provider, connection)
			if refreshErr != nil {
				response.Body.Close()
				fail(500, refreshErr.Error())
				return
			}
			if patch != nil {
				response.Body.Close()
				if ttsString(connection["apiKey"]) == "" {
					headers["Authorization"] = "Bearer " + ttsString(patch["accessToken"])
				}
				continue
			}
		}
		if contentType := response.Header.Get("Content-Type"); response.StatusCode >= 200 && response.StatusCode < 300 && isHTMLContentType(contentType) {
			response.Body.Close()
			nodeReply(w, 502, map[string]any{"success": false, "error": fmt.Sprintf("Provider %s returned HTML instead of a chat response (content-type: %s)", provider, contentType)})
			return
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			details, _ := io.ReadAll(response.Body)
			response.Body.Close()
			nodeReply(w, response.StatusCode, map[string]any{"success": false, "error": fmt.Sprintf("Provider error: %d", response.StatusCode), "details": string(details)})
			return
		}

		reader := bufio.NewReaderSize(response.Body, codexSSEPeekBytes)
		prefix, matched, accountFallback, message := codexPeekSSE(reader)
		if matched == "" {
			t.writeCodexResponse(w, source, stream, resolvedModel, io.MultiReader(bytes.NewReader(prefix), reader))
			response.Body.Close()
			return
		}
		response.Body.Close()
		if accountFallback {
			codexSSEErrorResponse(w, message)
			return
		}
		if sseAttempt >= codexRetryAttempts {
			codexSSEErrorResponse(w, message)
			return
		}
		sseAttempt++
		select {
		case <-r.Context().Done():
			return
		case <-time.After(codexRetryDelay):
		}
	}
}

func (t *TranslatorSend) writeCodexResponse(w http.ResponseWriter, source string, stream bool, model string, body io.Reader) {
	if source == "openai-responses" {
		if stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			_, _ = io.Copy(w, body)
			return
		}
		raw, err := io.ReadAll(body)
		if err != nil {
			nodeReply(w, 502, map[string]any{"error": map[string]any{"message": err.Error()}})
			return
		}
		result, err := foldResponsesStream(raw)
		if err != nil {
			nodeReply(w, 502, map[string]any{"error": map[string]any{"message": err.Error()}})
			return
		}
		nodeReply(w, 200, result)
		return
	}
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		conversion := &streamConversion{destination: w, source: "openai", target: "openai-responses", model: model, textBlock: -1, serverTool: -1}
		if _, err := io.Copy(conversion, body); err != nil {
			conversion.finish()
			return
		}
		conversion.finish()
		return
	}
	buffer := &bufferResponseWriter{}
	conversion := &streamConversion{destination: buffer, source: "openai", target: "openai-responses", model: model, textBlock: -1, serverTool: -1}
	if _, err := io.Copy(conversion, body); err != nil {
		conversion.finish()
	}
	conversion.finish()
	if conversion.err != nil {
		nodeReply(w, 502, map[string]any{"error": map[string]any{"message": conversion.err.Error()}})
		return
	}
	completion, err := foldOpenAIStream(buffer.body.Bytes(), model)
	if err != nil {
		nodeReply(w, 502, map[string]any{"error": map[string]any{"message": err.Error()}})
		return
	}
	nodeReply(w, 200, completion)
}

package handlers

// Porta de open-sse/rtk/headroom.js: comprime o corpo via proxy Headroom
// (/v1/compress). Fail-open em qualquer erro/timeout, mas com diagnóstico
// explícito. Ferramentas (tool_calls/tool_call_id/role tool e blocos
// tool_use/tool_result) nunca são reescritas: se o proxy alterar a estrutura,
// a compressão inteira é descartada.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type headroomSizeSnapshot struct {
	BodyBytes    int `json:"bodyBytes"`
	MessageBytes int `json:"messageBytes"`
}

type headroomDiagnostics struct {
	Before   *headroomSizeSnapshot
	After    *headroomSizeSnapshot
	Reason   string
	Endpoint string
}

func (d *headroomDiagnostics) setReason(reason string) {
	if d != nil && d.Reason == "" {
		d.Reason = reason
	}
}

func (d *headroomDiagnostics) reasonOr(fallback string) string {
	if d == nil || d.Reason == "" {
		return fallback
	}
	return d.Reason
}

func (s *TokenSaver) compressWithHeadroom(ctx context.Context, body map[string]any, cfg tokenSaverSettings, format, model string, diag *headroomDiagnostics) map[string]any {
	if !cfg.headroomEnabled {
		diag.setReason("disabled")
		return nil
	}
	if strings.TrimSpace(cfg.headroomURL) == "" {
		diag.setReason("missing proxy URL")
		return nil
	}
	if body == nil {
		diag.setReason("missing request body")
		return nil
	}
	diag.Before = captureHeadroomSize(body)

	switch format {
	case "claude":
		return s.headroomClaude(ctx, body, cfg, model, diag)
	case "openai-responses":
		return s.headroomResponses(ctx, body, cfg, model, diag)
	case "kiro":
		diag.setReason("unsupported kiro request shape")
		return nil
	}

	key := ""
	if _, ok := body["messages"].([]any); ok {
		key = "messages"
	} else if _, ok := body["input"].([]any); ok {
		key = "input"
	}
	if key == "" {
		diag.setReason(fmt.Sprintf("unsupported %s request shape", format))
		return nil
	}
	original, _ := body[key].([]any)
	compressed, stats, ok := s.callHeadroomCompress(ctx, cfg, original, model, diag)
	if !ok {
		return nil
	}
	merged, ok := mergeHeadroomMessages(original, compressed, diag)
	if !ok {
		return nil
	}
	body[key] = merged
	diag.After = captureHeadroomSize(body)
	return stats
}

func (s *TokenSaver) headroomClaude(ctx context.Context, body map[string]any, cfg tokenSaverSettings, model string, diag *headroomDiagnostics) map[string]any {
	// A tradução Claude→OpenAI→Claude reconstrói blocos. Só aplicamos quando
	// todo o histórico é texto: tool_use/tool_result e mídia ficariam sujeitos
	// a reescrita, o que viola a garantia de tools intactas.
	if reason := claudeHeadroomUnsafeReason(body); reason != "" {
		diag.setReason(reason)
		return nil
	}
	oai, err := translateSource(body, "claude")
	if err != nil {
		diag.setReason("Claude request did not translate: " + err.Error())
		return nil
	}
	messages, ok := oai["messages"].([]any)
	if !ok {
		diag.setReason("Claude request did not translate to messages[]")
		return nil
	}
	compressed, stats, ok := s.callHeadroomCompress(ctx, cfg, messages, model, diag)
	if !ok {
		return nil
	}
	merged, ok := mergeHeadroomMessages(messages, compressed, diag)
	if !ok {
		return nil
	}
	oai["messages"] = merged
	claudeBody, err := translateTarget(oai, "claude", model)
	if err != nil {
		diag.setReason("compressed messages did not translate back to Claude: " + err.Error())
		return nil
	}
	claudeMessages, ok := claudeBody["messages"].([]any)
	if !ok {
		diag.setReason("compressed Claude body missing messages[]")
		return nil
	}
	if claudeHeadroomHasToolBlocks(claudeMessages) {
		diag.setReason("proxy response introduced Claude tool blocks")
		return nil
	}
	// Substitui apenas messages: system original (cache_control incluso) fica.
	body["messages"] = claudeMessages
	diag.After = captureHeadroomSize(body)
	return stats
}

func (s *TokenSaver) headroomResponses(ctx context.Context, body map[string]any, cfg tokenSaverSettings, model string, diag *headroomDiagnostics) map[string]any {
	if headroomUnsafeResponsesInput(body) {
		diag.setReason("skipped: openai-responses tool/reasoning input is not safe to compress")
		return nil
	}
	intermediate, err := translateSource(body, "openai-responses")
	if err != nil {
		diag.setReason("openai-responses request did not translate: " + err.Error())
		return nil
	}
	messages, ok := intermediate["messages"].([]any)
	if !ok {
		diag.setReason("openai-responses request did not translate to messages[]")
		return nil
	}
	compressed, stats, ok := s.callHeadroomCompress(ctx, cfg, messages, model, diag)
	if !ok {
		return nil
	}
	merged, ok := mergeHeadroomMessages(messages, compressed, diag)
	if !ok {
		return nil
	}
	intermediate["messages"] = merged
	intermediate["input"] = nil // força reconstruir input a partir das mensagens
	responsesBody, err := openAIResponsesTarget(intermediate, model)
	if err != nil {
		diag.setReason("compressed messages did not translate back to Responses: " + err.Error())
		return nil
	}
	input, ok := responsesBody["input"].([]any)
	if !ok {
		diag.setReason("compressed Responses body missing input[]")
		return nil
	}
	body["input"] = input
	diag.After = captureHeadroomSize(body)
	return stats
}

func headroomUnsafeResponsesInput(body map[string]any) bool {
	input, ok := body["input"].([]any)
	if !ok {
		return false
	}
	for _, raw := range input {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if itemType := ttsString(item["type"]); itemType != "" && itemType != "message" {
			return true
		}
	}
	return false
}

func claudeHeadroomHasToolBlocks(messages []any) bool {
	for _, raw := range messages {
		message, _ := raw.(map[string]any)
		content, _ := message["content"].([]any)
		for _, rawBlock := range content {
			block, _ := rawBlock.(map[string]any)
			switch ttsString(block["type"]) {
			case "tool_use", "tool_result", "server_tool_use", "web_search_tool_result":
				return true
			}
		}
	}
	return false
}

func claudeHeadroomUnsafeReason(body map[string]any) string {
	messages, _ := body["messages"].([]any)
	for _, raw := range messages {
		message, _ := raw.(map[string]any)
		content, _ := message["content"].([]any)
		for _, rawBlock := range content {
			block, _ := rawBlock.(map[string]any)
			switch ttsString(block["type"]) {
			case "tool_use", "tool_result", "server_tool_use", "web_search_tool_result":
				return "skipped: claude tool blocks are not safe to compress"
			case "text":
				// seguro
			default:
				return "skipped: claude non-text content is not safe to compress"
			}
		}
	}
	return ""
}

func mergeHeadroomMessages(original, compressed []any, diag *headroomDiagnostics) ([]any, bool) {
	if len(original) != len(compressed) {
		diag.setReason("proxy response did not match message count")
		return nil, false
	}
	merged := make([]any, len(original))
	for index := range original {
		originalMessage, _ := original[index].(map[string]any)
		compressedMessage, _ := compressed[index].(map[string]any)
		if originalMessage == nil || compressedMessage == nil {
			diag.setReason("proxy response contained an invalid message")
			return nil, false
		}
		if ttsString(originalMessage["role"]) != ttsString(compressedMessage["role"]) {
			diag.setReason("proxy response did not preserve message order")
			return nil, false
		}
		// Mensagens de ferramenta e tool_calls são sempre as originais.
		if ttsString(originalMessage["role"]) == "tool" {
			merged[index] = originalMessage
			continue
		}
		if calls, exists := originalMessage["tool_calls"]; exists {
			compressedMessage["tool_calls"] = calls
		}
		for _, key := range []string{"tool_call_id", "name", "recipient_name"} {
			if value, exists := originalMessage[key]; exists {
				compressedMessage[key] = value
			}
		}
		merged[index] = compressedMessage
	}
	return merged, true
}

func (s *TokenSaver) callHeadroomCompress(ctx context.Context, cfg tokenSaverSettings, messages []any, model string, diag *headroomDiagnostics) ([]any, map[string]any, bool) {
	endpoint := buildHeadroomCompressEndpoint(cfg.headroomURL)
	diag.Endpoint = maskHeadroomEndpoint(endpoint)
	payload := map[string]any{"messages": messages, "model": model}
	if cfg.headroomUserMessages {
		payload["config"] = map[string]any{"compress_user_messages": true}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		diag.setReason("request encoding failed: " + err.Error())
		return nil, nil, false
	}
	timeout := cfg.headroomTimeout
	if timeout <= 0 {
		timeout = defaultHeadroomTimeout
	}
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		diag.setReason("request build failed: " + err.Error())
		return nil, nil, false
	}
	request.Header.Set("Content-Type", "application/json")
	client := s.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		diag.setReason("request failed: " + describeHeadroomError(err))
		return nil, nil, false
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, response.Body)
		diag.setReason(fmt.Sprintf("proxy returned HTTP %d", response.StatusCode))
		return nil, nil, false
	}
	var raw map[string]any
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		diag.setReason("proxy response was not valid JSON: " + err.Error())
		return nil, nil, false
	}
	messages, ok := raw["messages"].([]any)
	if !ok || messages == nil {
		diag.setReason("proxy response missing messages[]")
		return nil, nil, false
	}
	// Node usa o objeto inteiro como stats (tokens_before/tokens_saved no root).
	return messages, raw, true
}

func captureHeadroomSize(body map[string]any) *headroomSizeSnapshot {
	snapshot := &headroomSizeSnapshot{}
	if encoded, err := json.Marshal(body); err == nil {
		snapshot.BodyBytes = len(encoded)
	}
	var messages []any
	if value, ok := body["messages"].([]any); ok {
		messages = value
	} else if value, ok := body["input"].([]any); ok {
		messages = value
	}
	if encoded, err := json.Marshal(messages); err == nil {
		snapshot.MessageBytes = len(encoded)
	}
	return snapshot
}

func buildHeadroomCompressEndpoint(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" {
		parsed.Path = strings.TrimRight(parsed.Path, "/") + headroomCompressEndpoint
		parsed.Fragment = ""
		return parsed.String()
	}
	raw = strings.SplitN(trimmed, "#", 2)[0]
	base := raw
	query := ""
	if parts := strings.SplitN(raw, "?", 2); len(parts) == 2 {
		base, query = parts[0], parts[1]
	}
	endpoint := strings.TrimRight(base, "/") + headroomCompressEndpoint
	if query != "" {
		endpoint += "?" + query
	}
	return endpoint
}

func maskHeadroomEndpoint(raw string) string {
	if parsed, err := url.Parse(raw); err == nil {
		parsed.User = nil
		parsed.RawQuery = ""
		parsed.Fragment = ""
		return parsed.String()
	}
	return raw
}

var (
	headroomUserInfoPattern = regexp.MustCompile(`//[^/@\s]+@`)
	headroomQueryPattern    = regexp.MustCompile(`(https?://[^\s?#]+)[?#][^\s)]*`)
)

func describeHeadroomError(err error) string {
	text := ""
	if err != nil {
		text = err.Error()
	}
	text = headroomUserInfoPattern.ReplaceAllString(text, "//")
	text = headroomQueryPattern.ReplaceAllString(text, "$1")
	return text
}

func formatHeadroomLog(stats map[string]any) string {
	if stats == nil {
		return ""
	}
	before := tokenSaverInt(stats["tokens_before"], 0)
	after := tokenSaverInt(stats["tokens_after"], 0)
	delta := tokenSaverInt(stats["tokens_saved"], 0)
	pct := 0.0
	if before > 0 {
		pct = float64(delta) / float64(before) * 100
	}
	line := fmt.Sprintf("reported token delta=%d before=%d", delta, before)
	if after != 0 {
		line += fmt.Sprintf(" after=%d", after)
	}
	line += fmt.Sprintf(" (%.1f%%)", pct)
	return line
}

func formatHeadroomSizeLog(diag *headroomDiagnostics) string {
	if diag == nil || diag.Before == nil || diag.After == nil {
		return ""
	}
	return fmt.Sprintf("body=%dB→%dB messages=%dB→%dB", diag.Before.BodyBytes, diag.After.BodyBytes, diag.Before.MessageBytes, diag.After.MessageBytes)
}

func headroomPhantomSavings(stats map[string]any, diag *headroomDiagnostics) bool {
	if stats == nil || tokenSaverInt(stats["tokens_saved"], 0) <= 0 {
		return false
	}
	if diag == nil || diag.Before == nil || diag.After == nil {
		return false
	}
	before, after := diag.Before.BodyBytes, diag.After.BodyBytes
	if before <= 0 || after <= 0 {
		return false
	}
	return after >= int(float64(before)*0.95)
}

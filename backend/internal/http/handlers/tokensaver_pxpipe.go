package handlers

// Porta de open-sse/rtk/pxpipe.js: transforma contexto Claude volumoso em
// imagens. Só formato Claude, fail-open, com evento no dashboard e checagem de
// que tools/tool blocks permaneceram intactos.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"
)

func (s *TokenSaver) applyPxpipe(ctx context.Context, body map[string]any, cfg tokenSaverSettings, format, model, provider string) {
	startedAt := time.Now()
	summary := map[string]any{"applied": false}

	finish := func(reason string, extra map[string]any) {
		summary["reason"] = reason
		for key, value := range extra {
			summary[key] = value
		}
		if line := formatPxpipeLog(summary); line != "" {
			slog.Info("PXPIPE", "detail", line, "provider", provider, "model", model)
		} else {
			detail := ""
			if value := ttsString(summary["detail"]); value != "" {
				detail = " (" + value + ")"
			}
			slog.Debug("PXPIPE", "skipped", reason+detail, "provider", provider, "model", model)
		}
		s.appendPxpipeEvent(provider, model, summary)
	}

	if format != "claude" {
		finish("unsupported_format", map[string]any{"detail": format})
		return
	}
	transform := s.PxpipeTransform
	if transform == nil && s.Pxpipe != nil {
		transform = s.Pxpipe.Transform
	}
	if transform == nil {
		finish("not_installed", nil)
		return
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		finish("missing_body", map[string]any{"detail": err.Error()})
		return
	}
	originalChars := len(encoded)
	threshold := cfg.pxpipeMinChars
	if threshold <= 0 {
		threshold = defaultPxpipeMinChars
	}
	if originalChars < threshold {
		finish("below_threshold", map[string]any{"originalChars": originalChars, "threshold": threshold})
		return
	}

	timeout := cfg.pxpipeTimeout
	if timeout <= 0 {
		timeout = defaultPxpipeTimeout
	}
	transformCtx, cancel := context.WithTimeout(ctx, timeout)
	type transformOutcome struct {
		result *pxpipeTransformed
		err    error
	}
	done := make(chan transformOutcome, 1)
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				done <- transformOutcome{err: fmt.Errorf("transform panicked: %v", recovered)}
			}
		}()
		result, err := transform(transformCtx, encoded, model, threshold)
		done <- transformOutcome{result: result, err: err}
	}()

	var outcome transformOutcome
	select {
	case outcome = <-done:
	case <-transformCtx.Done():
		cancel()
		finish("timeout", map[string]any{"originalChars": originalChars, "durationMs": time.Since(startedAt).Milliseconds()})
		return
	}
	cancel()

	if outcome.err != nil {
		finish("transform_error", map[string]any{"detail": outcome.err.Error(), "originalChars": originalChars, "durationMs": time.Since(startedAt).Milliseconds()})
		return
	}
	if outcome.result == nil {
		finish("timeout", map[string]any{"originalChars": originalChars, "durationMs": time.Since(startedAt).Milliseconds()})
		return
	}
	if !outcome.result.Applied {
		reason := outcome.result.Reason
		if reason == "" {
			reason = "passthrough"
		}
		extra := map[string]any{"originalChars": originalChars, "durationMs": time.Since(startedAt).Milliseconds()}
		if outcome.result.Detail != "" {
			extra["detail"] = outcome.result.Detail
		}
		finish(reason, extra)
		return
	}

	var newBody map[string]any
	if err := json.Unmarshal(outcome.result.Body, &newBody); err != nil {
		finish("transform_error", map[string]any{"detail": "transformed body is not valid JSON: " + err.Error(), "originalChars": originalChars, "durationMs": time.Since(startedAt).Milliseconds()})
		return
	}
	if !pxpipeToolsPreserved(body, newBody) {
		finish("tools_modified", map[string]any{"detail": "transform changed tools[]", "originalChars": originalChars, "durationMs": time.Since(startedAt).Milliseconds()})
		return
	}
	if !pxpipeToolBlocksPreserved(body, newBody) {
		finish("tools_modified", map[string]any{"detail": "transform changed tool_use/tool_result blocks", "originalChars": originalChars, "durationMs": time.Since(startedAt).Milliseconds()})
		return
	}

	replaceMapContents(body, newBody)
	compressedEncoded, _ := json.Marshal(newBody)
	info := outcome.result.Info
	if info == nil {
		info = map[string]any{}
	}
	imagedChars := int(pxpipeNum(info["compressedChars"]))
	imageTokens := int(pxpipeNum(info["imageTokens"]))
	if imageTokens == 0 {
		if pixels := pxpipeNum(info["imagePixels"]); pixels > 0 {
			imageTokens = int(math.Round(pixels / 750))
		} else {
			imageTokens = int(pxpipeNum(info["imageCount"])) * 4761
		}
	}
	tokensBefore := int(pxpipeNum(info["baselineTokens"]))
	if tokensBefore == 0 {
		tokensBefore = int(math.Round(float64(originalChars) / tokenSaverEstCharsPerToken))
	}
	remaining := originalChars - imagedChars
	if remaining < 0 {
		remaining = 0
	}
	tokensAfter := int(math.Round(float64(remaining)/tokenSaverEstCharsPerToken)) + imageTokens
	tokensSaved := tokensBefore - tokensAfter
	if tokensSaved < 0 {
		tokensSaved = 0
	}
	savedPct := 0.0
	if tokensBefore > 0 {
		savedPct = math.Round(float64(tokensSaved)/float64(tokensBefore)*10000) / 100
	}
	cacheOwnsControl := false
	if cache := outcome.result.Cache; cache != nil {
		cacheOwnsControl = tokenSaverTruthy(cache["ownsCacheControl"])
	}
	summary = map[string]any{
		"applied":             true,
		"reason":              "applied",
		"originalChars":       originalChars,
		"compressedBodyChars": len(compressedEncoded),
		"imagedChars":         imagedChars,
		"imageCount":          int(pxpipeNum(info["imageCount"])),
		"imageBytes":          int(pxpipeNum(info["imageBytes"])),
		"tokensBeforeEst":     tokensBefore,
		"tokensAfterEst":      tokensAfter,
		"tokensSavedEst":      tokensSaved,
		"savedPct":            savedPct,
		"durationMs":          time.Since(startedAt).Milliseconds(),
		"cacheOwnsControl":    cacheOwnsControl,
	}
	finish("applied", nil)
}

func (s *TokenSaver) appendPxpipeEvent(provider, model string, summary map[string]any) {
	if s.PxpipeDataDir == "" {
		return
	}
	event := map[string]any{"provider": provider, "model": model}
	for key, value := range summary {
		event[key] = value
	}
	PxpipeAppendEvent(s.PxpipeDataDir, event)
}

func formatPxpipeLog(summary map[string]any) string {
	if summary == nil || !tokenSaverTruthy(summary["applied"]) {
		return ""
	}
	return fmt.Sprintf("imaged %dch → %d image(s) | est %d→%d tokens (-%s%%) | %dms",
		int(pxpipeNum(summary["imagedChars"])),
		int(pxpipeNum(summary["imageCount"])),
		int(pxpipeNum(summary["tokensBeforeEst"])),
		int(pxpipeNum(summary["tokensAfterEst"])),
		strconv.FormatFloat(pxpipeNum(summary["savedPct"]), 'f', -1, 64),
		int(pxpipeNum(summary["durationMs"])),
	)
}

func replaceMapContents(target, replacement map[string]any) {
	for key := range target {
		delete(target, key)
	}
	for key, value := range replacement {
		target[key] = value
	}
}

func pxpipeToolsPreserved(original, transformed map[string]any) bool {
	return pxpipeJSONEqual(original["tools"], transformed["tools"])
}

func pxpipeJSONEqual(left, right any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func pxpipeToolBlocksPreserved(original, transformed map[string]any) bool {
	return pxpipeJSONEqual(pxpipeClaudeToolBlocks(original), pxpipeClaudeToolBlocks(transformed))
}

func pxpipeClaudeToolBlocks(body map[string]any) []any {
	result := []any{}
	messages, _ := body["messages"].([]any)
	for _, rawMessage := range messages {
		message, _ := rawMessage.(map[string]any)
		content, _ := message["content"].([]any)
		for _, rawBlock := range content {
			block, _ := rawBlock.(map[string]any)
			switch ttsString(block["type"]) {
			case "tool_use", "tool_result", "server_tool_use", "web_search_tool_result":
				result = append(result, block)
			}
		}
	}
	return result
}

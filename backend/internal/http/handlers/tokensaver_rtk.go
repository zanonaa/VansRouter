package handlers

// Adapt message shapes only. All detection and compression runs in the official
// RTK CLI via stdin; no copied filters and no shell execution of tool content.
import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

const rtkMaxBodyBytes = 512 * 1024

type rtkHit struct {
	Shape, Filter string
	Saved         int
}
type rtkStats struct {
	BytesBefore, BytesAfter int
	Hits                    []rtkHit
}

func compressRTKMessages(body map[string]any) *rtkStats {
	if body == nil {
		return nil
	}
	if estimateRTKValueBytes(body) > rtkMaxBodyBytes {
		return nil
	}
	if _, ok := body["conversationState"].(map[string]any); ok {
		return compressRTKKiro(body)
	}
	items, ok := body["messages"].([]any)
	if !ok {
		items, ok = body["input"].([]any)
	}
	if !ok {
		return nil
	}

	stats := &rtkStats{}
	for _, raw := range items {
		msg, ok := raw.(map[string]any)
		if !ok || msg == nil {
			continue
		}

		// OpenAI Responses: {type:"function_call_output", output: string|[{type:"input_text",text}]}
		if ttsString(msg["type"]) == "function_call_output" {
			switch output := msg["output"].(type) {
			case string:
				msg["output"] = compressRTKText(output, stats, "openai-responses-string")
			case []any:
				for _, rawPart := range output {
					part, ok := rawPart.(map[string]any)
					if !ok || ttsString(part["type"]) != "input_text" {
						continue
					}
					if text, ok := part["text"].(string); ok {
						part["text"] = compressRTKText(text, stats, "openai-responses-array")
					}
				}
			}
			continue
		}

		if ttsString(msg["role"]) == "tool" {
			if content, ok := msg["content"].(string); ok {
				msg["content"] = compressRTKText(content, stats, "openai-tool")
				continue
			}
		}

		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}

		// OpenAI tool message com content em blocos de texto.
		if ttsString(msg["role"]) == "tool" {
			for _, rawPart := range content {
				part, ok := rawPart.(map[string]any)
				if !ok || ttsString(part["type"]) != "text" {
					continue
				}
				if text, ok := part["text"].(string); ok {
					part["text"] = compressRTKText(text, stats, "openai-tool-array")
				}
			}
			continue
		}

		for _, rawBlock := range content {
			block, ok := rawBlock.(map[string]any)
			if !ok || ttsString(block["type"]) != "tool_result" {
				continue
			}
			if isError, ok := block["is_error"].(bool); ok && isError {
				continue // preserva traces de erro
			}
			switch blockContent := block["content"].(type) {
			case string:
				block["content"] = compressRTKText(blockContent, stats, "claude-string")
			case []any:
				for _, rawPart := range blockContent {
					part, ok := rawPart.(map[string]any)
					if !ok || ttsString(part["type"]) != "text" {
						continue
					}
					if text, ok := part["text"].(string); ok {
						part["text"] = compressRTKText(text, stats, "claude-array")
					}
				}
			}
		}
	}
	return stats
}

// compressRTKKiro porta compressKiroFormat (history + currentMessage).
func compressRTKKiro(body map[string]any) *rtkStats {
	state, _ := body["conversationState"].(map[string]any)
	stats := &rtkStats{}
	all := []any{}
	if history, ok := state["history"].([]any); ok {
		all = append(all, history...)
	}
	if current, ok := state["currentMessage"]; ok && current != nil {
		all = append(all, current)
	}
	for _, raw := range all {
		msg, _ := raw.(map[string]any)
		user, _ := msg["userInputMessage"].(map[string]any)
		context, _ := user["userInputMessageContext"].(map[string]any)
		results, _ := context["toolResults"].([]any)
		for _, rawResult := range results {
			result, _ := rawResult.(map[string]any)
			if ttsString(result["status"]) == "error" {
				continue
			}
			parts, _ := result["content"].([]any)
			for _, rawPart := range parts {
				part, _ := rawPart.(map[string]any)
				if text, ok := part["text"].(string); ok {
					part["text"] = compressRTKText(text, stats, "kiro-tool-result")
				}
			}
		}
	}
	return stats
}

func compressRTKText(text string, stats *rtkStats, shape string) string {
	stats.BytesBefore += len(text)
	out := text
	if len(text) >= 500 && len(text) <= rtkMaxBodyBytes {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		cmd := exec.CommandContext(ctx, "rtk", "pipe")
		cmd.Stdin = strings.NewReader(text)
		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			slog.Warn("RTK official CLI unavailable or failed; tool result unchanged")
		} else if stdout.Len() > 0 && stdout.Len() < len(text) {
			out = stdout.String()
		}
		cancel()
	}
	stats.BytesAfter += len(out)
	if out != text {
		stats.Hits = append(stats.Hits, rtkHit{shape, "official-pipe", len(text) - len(out)})
	}
	return out
}
func formatRTKLog(stats *rtkStats) string {
	if stats == nil || len(stats.Hits) == 0 {
		return ""
	}
	saved := stats.BytesBefore - stats.BytesAfter
	pct := 0.0
	if stats.BytesBefore > 0 {
		pct = float64(saved) / float64(stats.BytesBefore) * 100
	}
	seen := map[string]bool{}
	filters := []string{}
	for _, hit := range stats.Hits {
		if seen[hit.Filter] {
			continue
		}
		seen[hit.Filter] = true
		filters = append(filters, hit.Filter)
	}
	return fmt.Sprintf("saved %dB / %dB (%.1f%%) via [%s] hits=%d", saved, stats.BytesBefore, pct, strings.Join(filters, ","), len(stats.Hits))
}

func estimateRTKValueBytes(value any) int {
	switch typed := value.(type) {
	case string:
		return len(typed)
	case map[string]any:
		total := 0
		for _, child := range typed {
			total += estimateRTKValueBytes(child)
		}
		return total
	case []any:
		total := 0
		for _, child := range typed {
			total += estimateRTKValueBytes(child)
		}
		return total
	default:
		return 0
	}
}

package handlers

import (
	"encoding/json"
	"fmt"
	"regexp"
	"unicode/utf16"
)

var jinaMetadataTitle = regexp.MustCompile(`(?mi)^\s*Title:\s*(.+)$`)
var jinaHeadingTitle = regexp.MustCompile(`(?m)^\s*#\s+(.+)$`)

func jsTextLength(text string) int { return len(utf16.Encode([]rune(text))) }
func truncateFetch(text string, max int) string {
	if max <= 0 {
		return text
	}
	units := utf16.Encode([]rune(text))
	if len(units) <= max {
		return text
	}
	return string(utf16.Decode(units[:max]))
}
func fetchFirst(payload map[string]any) map[string]any {
	entries, _ := payload["results"].([]any)
	if len(entries) == 0 {
		return map[string]any{}
	}
	entry, _ := entries[0].(map[string]any)
	return entry
}
func normalizeFetch(provider, target, format string, raw []byte, max int, cost any, duration int64) (map[string]any, error) {
	text := ""
	var title any
	var links any
	metadata := map[string]any{"author": nil, "published_at": nil, "language": nil}
	if provider == "jina-reader" {
		text = string(raw)
		match := jinaMetadataTitle.FindStringSubmatch(text)
		if match == nil {
			match = jinaHeadingTitle.FindStringSubmatch(text)
		}
		if len(match) > 1 {
			title = match[1]
		}
	} else {
		var payload map[string]any
		if err := json.Unmarshal(raw, &payload); err != nil {
			return nil, fmt.Errorf("%s returned invalid JSON", provider)
		}
		switch provider {
		case "firecrawl":
			data, _ := payload["data"].(map[string]any)
			text = ttsString(firstNonEmpty(data["markdown"], data["html"], data["text"]))
			meta, _ := data["metadata"].(map[string]any)
			title = meta["title"]
		case "tavily":
			text = ttsString(fetchFirst(payload)["raw_content"])
		case "exa":
			first := fetchFirst(payload)
			text = ttsString(first["text"])
			title = first["title"]
		case "tinyfish":
			failures, _ := payload["errors"].([]any)
			if len(failures) > 0 {
				failure, _ := failures[0].(map[string]any)
				return nil, fmt.Errorf("TinyFish fetch failed: %s", ttsString(firstNonEmpty(failure["error"], "unknown error")))
			}
			first := fetchFirst(payload)
			var ok bool
			text, ok = first["text"].(string)
			if !ok {
				return nil, fmt.Errorf("TinyFish returned no extractable content")
			}
			title = first["title"]
			links = first["links"]
			metadata = map[string]any{"author": first["author"], "published_at": first["published_date"], "language": first["language"]}
		default:
			return nil, fmt.Errorf("Unsupported provider: %s", provider)
		}
	}
	text = truncateFetch(text, max)
	result := map[string]any{"provider": provider, "url": target, "title": title, "content": map[string]any{"format": format, "text": text, "length": jsTextLength(text)}, "metadata": metadata, "usage": map[string]any{"fetch_cost_usd": cost}, "metrics": map[string]any{"response_time_ms": duration, "upstream_latency_ms": duration}}
	if _, ok := links.([]any); ok {
		result["links"] = links
	}
	return result, nil
}

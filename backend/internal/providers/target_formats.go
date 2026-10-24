package providers

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed target-formats.json
var targetFormatsJSON []byte

func TargetFormat(provider string) string {
	var formats map[string]string
	if err := json.Unmarshal(targetFormatsJSON, &formats); err != nil {
		panic(err)
	}
	if strings.HasPrefix(provider, "openai-compatible-") {
		if strings.Contains(provider, "responses") {
			return "openai-responses"
		}
		return "openai"
	}
	if strings.HasPrefix(provider, "anthropic-compatible-") {
		return "claude"
	}
	if format := formats[provider]; format != "" {
		return format
	}
	return "openai"
}

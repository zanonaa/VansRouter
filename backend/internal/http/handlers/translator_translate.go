package handlers

import (
	"net/http"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type TranslatorTranslate struct {
	Connections NodeConnections
}

func detectRequestFormat(body map[string]any) string {
	if body["input"] != nil && body["messages"] == nil {
		switch body["input"].(type) {
		case []any, string:
			return "openai-responses"
		}
	}
	if request, ok := body["request"].(map[string]any); ok && request["contents"] != nil && body["userAgent"] == "antigravity" {
		return "antigravity"
	}
	if _, ok := body["contents"].([]any); ok {
		return "gemini"
	}
	if body["stream_options"] != nil || body["response_format"] != nil || body["logprobs"] != nil || body["top_logprobs"] != nil || body["n"] != nil || body["presence_penalty"] != nil || body["frequency_penalty"] != nil || body["logit_bias"] != nil || nodeTruthy(body["user"]) {
		return "openai"
	}
	messages, _ := body["messages"].([]any)
	model := ttsString(body["model"])
	if len(messages) > 0 && !strings.Contains(model, "/") {
		claudeImage, openAIImage, claudeTool := false, false, false
		for _, raw := range messages {
			message, _ := raw.(map[string]any)
			content, _ := message["content"].([]any)
			for _, item := range content {
				entry, _ := item.(map[string]any)
				switch entry["type"] {
				case "image":
					source, _ := entry["source"].(map[string]any)
					claudeImage = claudeImage || source["type"] == "base64"
				case "image_url":
					image, _ := entry["image_url"].(map[string]any)
					openAIImage = openAIImage || nodeTruthy(image["url"])
				case "tool_use", "tool_result":
					claudeTool = true
				}
			}
		}
		if body["system"] != nil || nodeTruthy(body["anthropic_version"]) || claudeImage || claudeTool {
			return "claude"
		}
		if openAIImage {
			return "openai"
		}
	}
	if body["system"] != nil || nodeTruthy(body["anthropic_version"]) {
		return "claude"
	}
	return "openai"
}

func splitTranslatorModel(value string) (string, string) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	aliases := providers.AliasMap()
	provider := parts[0]
	if resolved := aliases[provider]; resolved != "" {
		provider = resolved
	}
	return provider, parts[1]
}

func (t *TranslatorTranslate) Post(w http.ResponseWriter, r *http.Request) {
	fail := func(status int, message string) {
		nodeReply(w, status, map[string]any{"success": false, "error": message})
	}
	input, err := poolInput(r)
	if err != nil {
		fail(500, err.Error())
		return
	}
	step, body := input["step"], input["body"]
	if !nodeTruthy(step) || !nodeTruthy(body) {
		fail(400, "Step and body required")
		return
	}
	payload, _ := body.(map[string]any)
	if payload == nil {
		fail(500, "Invalid request body")
		return
	}
	clientBody := payload
	if nested, ok := payload["body"].(map[string]any); ok {
		clientBody = nested
	}
	switch jsString(step) {
	case "1":
		provider, model := splitTranslatorModel(ttsString(clientBody["model"]))
		nodeReply(w, 200, map[string]any{"success": true, "result": map[string]any{"provider": provider, "model": model, "sourceFormat": detectRequestFormat(clientBody), "targetFormat": providers.TargetFormat(provider)}})
	case "2":
		source := detectRequestFormat(clientBody)
		converted, err := translateSource(clientBody, source)
		if err != nil {
			fail(501, err.Error())
			return
		}
		nodeReply(w, 200, map[string]any{"success": true, "result": map[string]any{"body": converted}})
	case "3":
		provider := ttsString(firstNonEmpty(payload["provider"], clientBody["provider"]))
		model := ttsString(firstNonEmpty(payload["model"], clientBody["model"]))
		if provider == "" || model == "" {
			fail(400, "provider and model required")
			return
		}
		format := providers.TargetFormat(provider)
		if format != "openai" && format != "claude" && format != "gemini" && format != "openai-responses" && format != "ollama" && format != "commandcode" {
			fail(501, "Translator conversion not ported for step 3")
			return
		}
		body, err := translateTarget(clientBody, format, model)
		if err != nil {
			fail(501, err.Error())
			return
		}
		if t.Connections == nil {
			fail(500, "Connection store unavailable")
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
		target, headers, err := translatorPlan(provider, model, clientBody["stream"] != false, connection)
		if err != nil {
			fail(501, err.Error())
			return
		}
		nodeReply(w, 200, map[string]any{"success": true, "result": map[string]any{"url": target, "headers": headers, "body": body}})
	default:
		fail(400, "Invalid step (1-3)")
	}
}

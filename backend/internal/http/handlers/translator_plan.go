package handlers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func translatorPlan(provider, model string, stream bool, connection map[string]any) (string, map[string]string, error) {
	config := providers.TransportFor(provider)
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	if strings.HasPrefix(provider, "openai-compatible-") {
		base := strings.TrimRight(ttsString(metadata["baseUrl"]), "/")
		if base != "" {
			config = &providers.Transport{BaseURL: base + "/chat/completions", Format: "openai"}
		}
	}
	if config == nil {
		return "", nil, fmt.Errorf("Translator executor not ported for provider: %s", provider)
	}
	target := config.BaseURL
	headers := map[string]string{"Content-Type": "application/json"}
	for k, v := range config.Headers {
		headers[k] = v
	}
	token := ttsString(firstNonEmpty(connection["apiKey"], connection["accessToken"]))
	if config.Format == "gemini" {
		action := ":generateContent"
		if stream {
			action = ":streamGenerateContent"
		}
		target = strings.TrimRight(target, "/") + "/" + url.PathEscape(model) + action
		parsed, err := url.Parse(target)
		if err != nil {
			return "", nil, err
		}
		query := parsed.Query()
		query.Set("key", token)
		if stream {
			query.Set("alt", "sse")
		}
		parsed.RawQuery = query.Encode()
		target = parsed.String()
	} else if !config.NoAuth {
		header, prefix := "Authorization", "Bearer "
		if config.Auth != nil {
			name, scheme := config.Auth.Header, config.Auth.Scheme
			if ttsString(connection["apiKey"]) != "" && config.Auth.APIKey != nil {
				name, scheme = config.Auth.APIKey.Header, config.Auth.APIKey.Scheme
			} else if ttsString(connection["accessToken"]) != "" && config.Auth.OAuth != nil {
				name, scheme = config.Auth.OAuth.Header, config.Auth.OAuth.Scheme
			}
			if name != "" {
				header = name
				if scheme != "bearer" {
					prefix = ""
				}
			}
		}
		headers[header] = prefix + token
	}
	return target, headers, nil
}

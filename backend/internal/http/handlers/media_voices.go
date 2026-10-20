package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// VoiceFetcher assina o contrato dos VOICE_FETCHERS do original.
type VoiceFetcher func(ctx context.Context, apiKey string) ([]map[string]any, error)

type MediaVoices struct {
	// Fetchers injetados; produção usa fetchers reais sem stub de sucesso.
	Fetchers map[string]VoiceFetcher
	// HTTP client compartilhado (sem proxy de ambiente).
	Client *http.Client
}

func (m MediaVoices) client() *http.Client {
	if m.Client != nil {
		return m.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func fetchEdgeVoices(ctx context.Context, client *http.Client) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://speech.platform.bing.com/consumer/speech/synthesize/readaloud/voices/list", nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("edge-tts voices: status %d", res.StatusCode)
	}
	var raw []map[string]any
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func fetchElevenVoices(ctx context.Context, client *http.Client, apiKey string) ([]map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.elevenlabs.io/v1/voices", nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("xi-api-key", apiKey)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("elevenlabs voices: status %d", res.StatusCode)
	}
	var payload struct {
		Voices []map[string]any `json:"voices"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Voices, nil
}

func (m MediaVoices) Get(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	provider := query.Get("provider")
	if provider == "" {
		provider = "edge-tts"
	}
	langFilter := query.Get("lang")
	apiKey := query.Get("apiKey")

	fetcher := m.Fetchers[provider]
	if fetcher == nil {
		nodeReply(w, 400, map[string]string{"error": fmt.Sprintf("Provider '%s' does not support voice listing", provider)})
		return
	}
	raw, err := fetcher(r.Context(), apiKey)
	if err != nil {
		nodeReply(w, 502, map[string]string{"error": err.Error()})
		return
	}

	voices := []map[string]any{}
	switch provider {
	case "local-device":
		for _, v := range raw {
			locale, _ := v["locale"].(string)
			lang, _ := v["lang"].(string)
			country, _ := v["country"].(string)
			gender, _ := v["gender"].(string)
			voices = append(voices, map[string]any{
				"id": v["id"], "name": v["name"], "locale": strings.ReplaceAll(locale, "_", "-"),
				"lang": lang, "country": country, "countryName": country, "langName": lang, "gender": gender,
			})
		}
	case "elevenlabs", "gemini":
		for _, v := range raw {
			locale := "en"
			if labels, ok := v["labels"].(map[string]any); ok {
				if language, ok := labels["language"].(string); ok && language != "" {
					locale = language
				}
			}
			lang := strings.SplitN(locale, "-", 2)[0]
			gender := ""
			if labels, ok := v["labels"].(map[string]any); ok {
				gender, _ = labels["gender"].(string)
			}
			voices = append(voices, map[string]any{
				"id": v["voice_id"], "name": v["name"], "locale": locale, "lang": lang,
				"country": "", "countryName": "", "langName": lang, "gender": gender, "category": v["category"],
			})
		}
	default:
		for _, v := range raw {
			locale, _ := v["Locale"].(string)
			parts := strings.SplitN(locale, "-", 2)
			lang, country := parts[0], ""
			if len(parts) == 2 {
				country = parts[1]
			}
			name, _ := v["FriendlyName"].(string)
			if name == "" {
				name, _ = v["ShortName"].(string)
			}
			name = strings.ReplaceAll(name, "Microsoft ", "")
			name = regexpReplace(name, " Online (Natural) - ", " (")
			voices = append(voices, map[string]any{
				"id": v["ShortName"], "name": name, "locale": locale, "lang": lang,
				"country": country, "countryName": country, "langName": lang, "gender": v["Gender"],
			})
		}
	}

	if langFilter != "" {
		filtered := voices[:0]
		for _, v := range voices {
			if v["lang"] == langFilter {
				filtered = append(filtered, v)
			}
		}
		voices = filtered
	}

	byLang := map[string]any{}
	order := []string{}
	for _, v := range voices {
		key, _ := v["lang"].(string)
		group, ok := byLang[key].(map[string]any)
		if !ok {
			group = map[string]any{"code": key, "name": v["langName"], "voices": []map[string]any{}}
			byLang[key] = group
			order = append(order, key)
		}
		group["voices"] = append(group["voices"].([]map[string]any), v)
	}
	sort.Slice(order, func(i, j int) bool {
		return fmt.Sprint(byLang[order[i]].(map[string]any)["name"]) < fmt.Sprint(byLang[order[j]].(map[string]any)["name"])
	})
	languages := []map[string]any{}
	for _, key := range order {
		languages = append(languages, byLang[key].(map[string]any))
	}
	nodeReply(w, 200, map[string]any{"voices": voices, "languages": languages, "byLang": byLang})
}

func regexpReplace(s, pattern, replacement string) string {
	// Evita dependência regexp no mapa quente; substituição literal do sufixo original.
	for {
		idx := strings.Index(s, pattern)
		if idx < 0 {
			return s
		}
		s = s[:idx] + replacement + s[idx+len(pattern):]
	}
}

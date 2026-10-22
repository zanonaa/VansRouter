package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// AudioVoices adapta os endpoints internos de vozes ao formato OpenAI.
// A chamada é feita em processo: não há requisição HTTP circular.
type AudioVoices struct {
	TTS   TTSVoices
	Media MediaVoices
}

type captureResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *captureResponse) Header() http.Header {
	if c.header == nil {
		c.header = http.Header{}
	}
	return c.header
}
func (c *captureResponse) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}
func (c *captureResponse) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	return c.body.Write(p)
}

func providerAlias(id string) string {
	for _, entry := range providers.Catalog() {
		if entry.ID == id && entry.Alias != nil && *entry.Alias != "" {
			return *entry.Alias
		}
	}
	return id
}

func (a AudioVoices) internal(r *http.Request, provider, lang string) (int, map[string]any) {
	target := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(r.Context())
	query := target.URL.Query()
	if lang != "" {
		query.Set("lang", lang)
	}
	target.URL.RawQuery = query.Encode()
	var captured captureResponse
	switch provider {
	case "elevenlabs":
		a.TTS.ElevenLabs(&captured, target)
	case "deepgram":
		a.TTS.Deepgram(&captured, target)
	case "inworld":
		a.TTS.Inworld(&captured, target)
	default:
		query.Set("provider", provider)
		target.URL.RawQuery = query.Encode()
		a.Media.Get(&captured, target)
		var payload map[string]any
		if json.Unmarshal(captured.body.Bytes(), &payload) != nil {
			payload = map[string]any{"error": "Failed"}
		}
		return captured.status, payload
	}
	var payload map[string]any
	if json.Unmarshal(captured.body.Bytes(), &payload) != nil {
		payload = map[string]any{"error": "Failed"}
	}
	return captured.status, payload
}

func (a AudioVoices) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	provider := r.URL.Query().Get("provider")
	supported := map[string]bool{"elevenlabs": true, "deepgram": true, "inworld": true, "edge-tts": true, "local-device": true}
	if !supported[provider] {
		names := []string{"deepgram", "edge-tts", "elevenlabs", "inworld", "local-device"}
		nodeReply(w, 400, map[string]any{"error": map[string]any{"message": "provider must be one of: " + strings.Join(names, ", "), "type": "invalid_request_error"}})
		return
	}
	lang := r.URL.Query().Get("lang")
	status, payload := a.internal(r, provider, lang)
	if status < 200 || status >= 300 || payload["error"] != nil {
		message := ttsString(payload["error"])
		if message == "" {
			message = "Upstream " + jsString(status)
		}
		if status < 400 {
			status = 502
		}
		nodeReply(w, status, map[string]any{"error": map[string]any{"message": message, "type": "server_error"}})
		return
	}
	raw := []map[string]any{}
	if lang != "" {
		if voices, ok := payload["voices"].([]any); ok {
			for _, item := range voices {
				if voice, ok := item.(map[string]any); ok {
					raw = append(raw, voice)
				}
			}
		}
	} else if groups, ok := payload["byLang"].(map[string]any); ok {
		for _, group := range groups {
			entry, _ := group.(map[string]any)
			voices, _ := entry["voices"].([]any)
			for _, item := range voices {
				if voice, ok := item.(map[string]any); ok {
					raw = append(raw, voice)
				}
			}
		}
	}
	alias := providerAlias(provider)
	data := []map[string]any{}
	for _, voice := range raw {
		id := ttsString(voice["id"])
		data = append(data, map[string]any{
			"id": id, "name": ttsString(voice["name"]), "lang": ttsString(voice["lang"]),
			"gender": ttsString(voice["gender"]), "model": alias + "/" + id,
		})
	}
	nodeReply(w, 200, map[string]any{"object": "list", "data": data})
}

func AudioVoicesOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.WriteHeader(http.StatusNoContent)
}

package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

var styledSpeechPrompt = regexp.MustCompile(`:\s`)

func geminiSpeechModelVoice(input string) (string, string) {
	known := []string{}
	seen := map[string]bool{}
	for _, entry := range providers.ModelInfo() {
		if entry["provider"] == "gemini" && entry["kind"] == "tts" {
			id := strings.TrimPrefix(ttsString(entry["id"]), "gemini/")
			if !seen[id] {
				known = append(known, id)
				seen[id] = true
			}
		}
	}
	fallback := "gemini-3.1-flash-tts-preview"
	if len(known) > 0 {
		fallback = known[0]
	}
	if input == "" {
		return fallback, "Kore"
	}
	for _, id := range known {
		if input == id {
			return id, "Kore"
		}
		if strings.HasPrefix(input, id+"/") {
			return id, strings.TrimPrefix(input, id+"/")
		}
	}
	return fallback, input
}
func pcmSpeechWAV(pcm []byte) []byte {
	output := make([]byte, 44+len(pcm))
	copy(output, "RIFF")
	binary.LittleEndian.PutUint32(output[4:], uint32(36+len(pcm)))
	copy(output[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(output[16:], 16)
	binary.LittleEndian.PutUint16(output[20:], 1)
	binary.LittleEndian.PutUint16(output[22:], 1)
	binary.LittleEndian.PutUint32(output[24:], 24000)
	binary.LittleEndian.PutUint32(output[28:], 48000)
	binary.LittleEndian.PutUint16(output[32:], 2)
	binary.LittleEndian.PutUint16(output[34:], 16)
	copy(output[36:], "data")
	binary.LittleEndian.PutUint32(output[40:], uint32(len(pcm)))
	copy(output[44:], pcm)
	return output
}
func (g *Gateway) synthesizeGemini(w http.ResponseWriter, r *http.Request, model, token string, config *providers.MediaConfig, body map[string]any) {
	model, voice := geminiSpeechModelVoice(model)
	text := strings.TrimSpace(ttsString(body["input"]))
	if text == "" {
		g.webError(w, 400, "Missing required field: input")
		return
	}
	if !styledSpeechPrompt.MatchString(text) {
		language := ttsString(body["language"])
		if language != "" {
			text = "Say in " + language + ": " + text
		} else {
			text = "Say: " + text
		}
	}
	payload := map[string]any{"contents": []any{map[string]any{"parts": []any{map[string]any{"text": text}}}}, "generationConfig": map[string]any{"responseModalities": []any{"AUDIO"}, "speechConfig": map[string]any{"voiceConfig": map[string]any{"prebuiltVoiceConfig": map[string]any{"voiceName": voice}}}}}
	encoded, err := json.Marshal(payload)
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()
	target := strings.TrimRight(config.BaseURL, "/") + "/" + url.PathEscape(model) + ":generateContent?key=" + url.QueryEscape(token)
	request, err := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(encoded))
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	var data map[string]any
	json.Unmarshal(raw, &data)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		failure, _ := data["error"].(map[string]any)
		g.webError(w, 502, ttsString(firstNonEmpty(failure["message"], fmt.Sprintf("Gemini TTS failed: %d", response.StatusCode))))
		return
	}
	candidates, _ := data["candidates"].([]any)
	candidate := map[string]any{}
	if len(candidates) > 0 {
		candidate, _ = candidates[0].(map[string]any)
	}
	content, _ := candidate["content"].(map[string]any)
	parts, _ := content["parts"].([]any)
	audio := ""
	for _, raw := range parts {
		part, _ := raw.(map[string]any)
		inline, _ := part["inlineData"].(map[string]any)
		if value := ttsString(inline["data"]); value != "" {
			audio = value
			break
		}
	}
	if audio == "" {
		feedback, _ := data["promptFeedback"].(map[string]any)
		reason := ttsString(firstNonEmpty(candidate["finishReason"], feedback["blockReason"], "unknown"))
		g.webError(w, 502, fmt.Sprintf("Gemini TTS returned no audio (finishReason: %s, voice: %s, model: %s)", reason, voice, model))
		return
	}
	pcm, err := base64.StdEncoding.DecodeString(audio)
	if err != nil {
		g.webError(w, 502, "Invalid Gemini PCM audio encoding")
		return
	}
	wav := pcmSpeechWAV(pcm)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.URL.Query().Get("response_format") == "json" {
		nodeReply(w, 200, map[string]any{"audio": base64.StdEncoding.EncodeToString(wav), "format": "wav"})
		return
	}
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", strconv.Itoa(len(wav)))
	w.Write(wav)
}

package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (g *Gateway) synthesizeXiaomi(w http.ResponseWriter, r *http.Request, model, token string, body map[string]any) {
	if token == "" {
		g.webError(w, 400, "xiaomi-mimo API key required")
		return
	}
	text := strings.TrimSpace(ttsString(body["input"]))
	modelID, voice := "mimo-v2.5-tts", "mimo_default"
	if model != "" {
		if strings.Contains(model, "/") {
			parts := strings.SplitN(model, "/", 2)
			modelID, voice = parts[0], parts[1]
		} else if model == modelID {
			voice = "mimo_default"
		} else {
			voice = model
		}
	}
	messages := []any{}
	instructions := []string{}
	if language := strings.TrimSpace(ttsString(body["language"])); language != "" {
		instructions = append(instructions, "Speak in "+language+".")
	}
	if style := strings.TrimSpace(ttsString(body["style"])); style != "" {
		instructions = append(instructions, style)
	}
	if len(instructions) > 0 {
		messages = append(messages, map[string]any{"role": "user", "content": strings.Join(instructions, " ")})
	}
	messages = append(messages, map[string]any{"role": "assistant", "content": text})
	encoded, err := json.Marshal(map[string]any{"model": modelID, "stream": false, "messages": messages, "audio": map[string]any{"format": "wav", "voice": voice}})
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "POST", "https://api.xiaomimimo.com/v1/chat/completions", bytes.NewReader(encoded))
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
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
		message := ttsString(failure["message"])
		if message == "" {
			message = fmt.Sprintf("MiMo TTS request failed (%d)", response.StatusCode)
		} else {
			message = fmt.Sprintf("MiMo TTS request failed (%d): %s", response.StatusCode, message)
		}
		g.webError(w, 502, message)
		return
	}
	choices, _ := data["choices"].([]any)
	choice := map[string]any{}
	if len(choices) > 0 {
		choice, _ = choices[0].(map[string]any)
	}
	message, _ := choice["message"].(map[string]any)
	audio, _ := message["audio"].(map[string]any)
	encodedAudio := ttsString(audio["data"])
	if encodedAudio == "" {
		g.webError(w, 502, ttsString(firstNonEmpty(data["error"], "MiMo TTS returned no audio")))
		return
	}
	decoded, err := base64.StdEncoding.DecodeString(encodedAudio)
	if err != nil {
		g.webError(w, 502, "Invalid MiMo audio encoding")
		return
	}
	format := ttsString(firstNonEmpty(audio["format"], "wav"))
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.URL.Query().Get("response_format") == "json" {
		nodeReply(w, 200, map[string]any{"audio": encodedAudio, "format": format})
		return
	}
	w.Header().Set("Content-Type", "audio/"+format)
	w.Header().Set("Content-Length", strconv.Itoa(len(decoded)))
	w.Write(decoded)
}

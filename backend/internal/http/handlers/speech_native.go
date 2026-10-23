package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func speechPlan(provider, model, text, token string, cfg *providers.MediaConfig) (string, map[string]string, map[string]any, string, error) {
	endpoint := cfg.BaseURL
	headers := map[string]string{"Content-Type": "application/json"}
	payload := map[string]any{}
	format := "mp3"
	modelID, voice := "", model
	if parts := strings.SplitN(model, "/", 2); len(parts) == 2 {
		modelID, voice = parts[0], parts[1]
	}
	switch cfg.Format {
	case "openai", "openai-speech":
		if provider == "openai" {
			if modelID == "" {
				modelID = "tts-1"
			}
		} else if modelID == "" {
			modelID = model
			voice = "alloy"
		}
		payload = map[string]any{"model": modelID, "voice": voice, "input": text}
		headers["Authorization"] = "Bearer " + token
	case "elevenlabs":
		if modelID == "" {
			modelID = "eleven_flash_v2_5"
		}
		endpoint = strings.TrimRight(endpoint, "/") + "/" + url.PathEscape(voice)
		headers["xi-api-key"] = token
		payload = map[string]any{"text": text, "model_id": modelID, "voice_settings": map[string]any{"stability": 0.5, "similarity_boost": 0.75}}
	case "fish-audio":
		if modelID == "" {
			modelID = model
			voice = ""
		}
		headers["Authorization"] = "Bearer " + token
		headers["model"] = modelID
		payload = map[string]any{"text": text, "format": "mp3"}
		if voice != "" {
			payload["reference_id"] = voice
		}
	case "cartesia":
		if modelID == "" {
			modelID = model
			voice = ""
		}
		headers["X-API-Key"] = token
		headers["Cartesia-Version"] = "2024-06-10"
		payload = map[string]any{"model_id": modelID, "transcript": text, "output_format": map[string]any{"container": "mp3", "bit_rate": 128000, "sample_rate": 44100}}
		if voice != "" {
			payload["voice"] = map[string]any{"mode": "id", "id": voice}
		}
	case "inworld":
		if modelID == "" {
			modelID = model
			voice = "Alex"
		}
		headers["Authorization"] = "Basic " + token
		payload = map[string]any{"text": text, "voiceId": voice, "modelId": modelID, "audioConfig": map[string]any{"audioEncoding": "MP3"}}
	case "minimax-tts":
		if modelID == "" {
			modelID = model
			voice = "English_expressive_narrator"
		}
		if modelID == "" {
			modelID = "speech-2.8-hd"
		}
		headers["Authorization"] = "Bearer " + token
		payload = map[string]any{"model": modelID, "text": text, "stream": false, "language_boost": "auto", "output_format": "hex", "voice_setting": map[string]any{"voice_id": voice, "speed": 1, "vol": 1, "pitch": 0}, "audio_setting": map[string]any{"sample_rate": 32000, "bitrate": 128000, "format": "mp3", "channel": 1}}
	case "playht":
		parts := strings.Split(token, ":")
		user, key := "", token
		if len(parts) > 1 {
			user, key = parts[0], parts[1]
		}
		if modelID == "" {
			modelID = model
			voice = ""
		}
		if modelID == "" {
			modelID = "PlayDialog"
		}
		if voice == "" {
			voice = "s3://voice-cloning-zero-shot/d9ff78ba-d016-47f6-b0ef-dd630f59414e/female-cs/manifest.json"
		}
		headers["Accept"] = "audio/mpeg"
		headers["X-USER-ID"] = user
		headers["Authorization"] = "Bearer " + key
		payload = map[string]any{"text": text, "voice": voice, "voice_engine": modelID, "output_format": "mp3", "speed": 1}
	case "coqui":
		payload = map[string]any{"text": text}
		if voice != "" {
			payload["speaker_id"] = voice
		}
		format = "wav"
	case "tortoise":
		payload = map[string]any{"text": text, "voice": voice}
		format = "wav"
	default:
		return "", nil, nil, "", fmt.Errorf("TTS adapter not implemented: %s", cfg.Format)
	}
	return endpoint, headers, payload, format, nil
}

func (g *Gateway) synthesizeNative(w http.ResponseWriter, r *http.Request, provider, model, token string, config *providers.MediaConfig, body map[string]any) {
	endpoint, headers, payload, format, err := speechPlan(provider, model, strings.TrimSpace(ttsString(body["input"])), token, config)
	if err != nil {
		g.webError(w, 501, err.Error())
		return
	}
	if config.Format == "openai-speech" {
		payload["response_format"] = "mp3"
		payload["speed"] = 1.0
		if requested := ttsString(body["response_format"]); requested != "" && requested != "json" {
			payload["response_format"] = requested
			format = requested
		}
		if speed, ok := body["speed"]; ok {
			payload["speed"] = speed
		}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(encoded))
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	for k, v := range headers {
		request.Header.Set(k, v)
	}
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
	audio, err := io.ReadAll(response.Body)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := string(audio)
		var payload map[string]any
		if json.Unmarshal(audio, &payload) == nil {
			nested, _ := payload["error"].(map[string]any)
			detail, _ := payload["detail"].(map[string]any)
			message = ttsString(firstNonEmpty(nested["message"], payload["message"], detail["message"], payload["detail"], message))
		}
		if message == "" {
			message = fmt.Sprintf("Upstream error (%d)", response.StatusCode)
		}
		g.webError(w, 502, message)
		return
	}
	if config.Format == "inworld" {
		var payload map[string]any
		if json.Unmarshal(audio, &payload) != nil || ttsString(payload["audioContent"]) == "" {
			g.webError(w, 502, "Inworld TTS returned no audio")
			return
		}
		audio, err = base64.StdEncoding.DecodeString(ttsString(payload["audioContent"]))
		if err != nil {
			g.webError(w, 502, "Invalid Inworld audio encoding")
			return
		}
	}
	if config.Format == "minimax-tts" {
		var payload map[string]any
		if json.Unmarshal(audio, &payload) != nil {
			g.webError(w, 502, "MiniMax TTS returned no audio")
			return
		}
		base, _ := firstNonEmpty(payload["base_resp"], payload["baseResp"]).(map[string]any)
		if usageNumber(firstNonEmpty(base["status_code"], base["statusCode"])) != 0 {
			g.webError(w, 502, ttsString(firstNonEmpty(base["status_msg"], base["statusMsg"], payload["message"], "MiniMax TTS upstream error")))
			return
		}
		data, _ := payload["data"].(map[string]any)
		encoded := strings.TrimSpace(ttsString(data["audio"]))
		if encoded == "" {
			g.webError(w, 502, "MiniMax TTS returned no audio")
			return
		}
		audio, err = hex.DecodeString(encoded)
		if err != nil {
			g.webError(w, 502, "MiniMax TTS returned invalid audio")
			return
		}
		extra, _ := firstNonEmpty(payload["extra_info"], payload["extraInfo"]).(map[string]any)
		if returned := ttsString(firstNonEmpty(extra["audio_format"], extra["audioFormat"])); returned != "" {
			format = returned
		}
	}
	if config.Format == "elevenlabs" && len(audio) < 1024 {
		g.webError(w, 502, "ElevenLabs TTS returned empty audio")
		return
	}
	if config.Format != "openai" && config.Format != "inworld" && config.Format != "elevenlabs" && config.Format != "minimax-tts" && len(audio) < 100 {
		g.webError(w, 502, "Upstream returned empty audio")
		return
	}
	ct := response.Header.Get("Content-Type")
	if strings.Contains(ct, "wav") {
		format = "wav"
	} else if strings.Contains(ct, "ogg") {
		format = "ogg"
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.URL.Query().Get("response_format") == "json" {
		nodeReply(w, 200, map[string]any{"audio": base64.StdEncoding.EncodeToString(audio), "format": format})
		return
	}
	w.Header().Set("Content-Type", "audio/"+format)
	w.Header().Set("Content-Length", strconv.Itoa(len(audio)))
	w.WriteHeader(200)
	w.Write(audio)
}

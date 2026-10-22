package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func audioContentType(file *multipart.FileHeader) string {
	declared := strings.ToLower(file.Header.Get("Content-Type"))
	if strings.HasPrefix(declared, "audio/") {
		return declared
	}
	mime := map[string]string{".mp3": "audio/mpeg", ".mp4": "audio/mp4", ".m4a": "audio/mp4", ".wav": "audio/wav", ".ogg": "audio/ogg", ".flac": "audio/flac", ".webm": "audio/webm", ".aac": "audio/aac", ".opus": "audio/opus"}[strings.ToLower(filepath.Ext(file.Filename))]
	if mime == "" {
		return "application/octet-stream"
	}
	return mime
}

func (g *Gateway) transcribeGemini(w http.ResponseWriter, r *http.Request, config *providers.MediaConfig, file multipart.File, header *multipart.FileHeader, model, token string) {
	audio, err := io.ReadAll(file)
	if err != nil {
		g.webError(w, 400, err.Error())
		return
	}
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if prompt == "" {
		prompt = "Generate a transcript of the speech. Return only the transcribed text, no commentary."
	}
	if lang := strings.TrimSpace(r.FormValue("language")); lang != "" {
		prompt += " Language: " + lang + "."
	}
	payload := map[string]any{"contents": []any{map[string]any{"parts": []any{map[string]any{"text": prompt}, map[string]any{"inline_data": map[string]any{"mime_type": audioContentType(header), "data": base64.StdEncoding.EncodeToString(audio)}}}}}}
	encoded, err := json.Marshal(payload)
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	target := strings.TrimRight(config.BaseURL, "/") + "/" + url.PathEscape(model) + ":generateContent?key=" + url.QueryEscape(token)
	request, err := http.NewRequestWithContext(r.Context(), "POST", target, bytes.NewReader(encoded))
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	g.transcriptionResult(w, request, "gemini-stt")
}

func (g *Gateway) transcriptionResult(w http.ResponseWriter, request *http.Request, format string) {
	ctx, cancel := context.WithTimeout(request.Context(), 300*time.Second)
	defer cancel()
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request.WithContext(ctx))
	if err != nil {
		status := 502
		if ctx.Err() == context.DeadlineExceeded {
			status = 504
		}
		g.webError(w, status, err.Error())
		return
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	var data map[string]any
	parseErr := json.Unmarshal(raw, &data)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		message := string(raw)
		if parseErr == nil {
			nested, _ := data["error"].(map[string]any)
			if detail := firstNonEmpty(nested["message"], data["error"], data["message"]); detail != nil {
				if text, ok := detail.(string); ok {
					message = text
				} else {
					encoded, _ := json.Marshal(detail)
					message = string(encoded)
				}
			}
		}
		if message == "" {
			message = fmt.Sprintf("Upstream error (%d)", response.StatusCode)
		}
		g.webError(w, response.StatusCode, message)
		return
	}
	if parseErr != nil {
		g.webError(w, 502, "Invalid transcription response JSON")
		return
	}
	text := ""
	switch format {
	case "deepgram":
		results, _ := data["results"].(map[string]any)
		channels, _ := results["channels"].([]any)
		if len(channels) > 0 {
			channel, _ := channels[0].(map[string]any)
			alternatives, _ := channel["alternatives"].([]any)
			if len(alternatives) > 0 {
				first, _ := alternatives[0].(map[string]any)
				text = ttsString(first["transcript"])
			}
		}
	case "huggingface-asr":
		text = ttsString(data["text"])
	case "gemini-stt":
		candidates, _ := data["candidates"].([]any)
		if len(candidates) > 0 {
			candidate, _ := candidates[0].(map[string]any)
			content, _ := candidate["content"].(map[string]any)
			parts, _ := content["parts"].([]any)
			for _, raw := range parts {
				part, _ := raw.(map[string]any)
				if value, ok := part["text"].(string); ok {
					text += value
				}
			}
		}
	default:
		g.webError(w, 501, "STT response adapter not implemented: "+format)
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	nodeReply(w, 200, map[string]any{"text": text})
}

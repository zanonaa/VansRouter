package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func waitTranscriptionPoll(ctx context.Context) error {
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (g *Gateway) assemblyJSON(ctx context.Context, method, target, token, contentType string, body io.Reader) (int, map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return 0, nil, err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return response.StatusCode, nil, err
	}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return response.StatusCode, nil, fmt.Errorf("Invalid AssemblyAI JSON response")
	}
	return response.StatusCode, data, nil
}
func (g *Gateway) transcribeAssemblyAI(w http.ResponseWriter, r *http.Request, config *providers.MediaConfig, file multipart.File, model, token string, wait func(context.Context) error) {
	ctx, cancel := context.WithTimeout(r.Context(), 300*time.Second)
	defer cancel()
	fail := func(status int, err error) {
		if ctx.Err() != nil {
			g.webError(w, 504, "AssemblyAI timeout after 120s")
			return
		}
		if status < 400 {
			status = 502
		}
		g.webError(w, status, err.Error())
	}
	status, upload, err := g.assemblyJSON(ctx, "POST", "https://api.assemblyai.com/v2/upload", token, "application/octet-stream", file)
	if err != nil {
		fail(status, err)
		return
	}
	if status < 200 || status >= 300 {
		fail(status, fmt.Errorf("%s", ttsString(firstNonEmpty(upload["error"], "AssemblyAI upload failed"))))
		return
	}
	uploadURL := ttsString(upload["upload_url"])
	if uploadURL == "" {
		fail(502, fmt.Errorf("AssemblyAI returned no upload URL"))
		return
	}
	payload, _ := json.Marshal(map[string]any{"audio_url": uploadURL, "speech_models": []any{model}, "language_detection": true})
	status, job, err := g.assemblyJSON(ctx, "POST", config.BaseURL, token, "application/json", bytes.NewReader(payload))
	if err != nil {
		fail(status, err)
		return
	}
	if status < 200 || status >= 300 {
		fail(status, fmt.Errorf("%s", ttsString(firstNonEmpty(job["error"], "AssemblyAI submission failed"))))
		return
	}
	id := ttsString(job["id"])
	if id == "" {
		fail(502, fmt.Errorf("AssemblyAI returned no transcription id"))
		return
	}
	pollCtx, pollCancel := context.WithTimeout(ctx, 120*time.Second)
	defer pollCancel()
	for {
		if err := wait(pollCtx); err != nil {
			g.webError(w, 504, "AssemblyAI timeout after 120s")
			return
		}
		status, result, err := g.assemblyJSON(pollCtx, "GET", strings.TrimRight(config.BaseURL, "/")+"/"+url.PathEscape(id), token, "", nil)
		if pollCtx.Err() != nil {
			g.webError(w, 504, "AssemblyAI timeout after 120s")
			return
		}
		if err != nil {
			fail(status, err)
			return
		}
		if status < 200 || status >= 300 {
			continue
		}
		switch result["status"] {
		case "completed":
			w.Header().Set("Access-Control-Allow-Origin", "*")
			nodeReply(w, 200, map[string]any{"text": ttsString(result["text"])})
			return
		case "error":
			g.webError(w, 500, ttsString(firstNonEmpty(result["error"], "AssemblyAI failed")))
			return
		}
	}
}

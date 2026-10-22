package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"
)

// ModelPing invokes the local gateway, not an arbitrary client-supplied URL.
// Headers supplies the same internal credentials used by dashboard requests.
type ModelPing struct {
	BaseURL string
	Client  *http.Client
	Headers func(context.Context) (http.Header, error)
	Timeout time.Duration
}

func silentTestWAV() []byte {
	buffer := make([]byte, 44+8000)
	copy(buffer, "RIFF")
	binary.LittleEndian.PutUint32(buffer[4:], uint32(len(buffer)-8))
	copy(buffer[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(buffer[16:], 16)
	binary.LittleEndian.PutUint16(buffer[20:], 1)
	binary.LittleEndian.PutUint16(buffer[22:], 1)
	binary.LittleEndian.PutUint32(buffer[24:], 16000)
	binary.LittleEndian.PutUint32(buffer[28:], 32000)
	binary.LittleEndian.PutUint16(buffer[32:], 2)
	binary.LittleEndian.PutUint16(buffer[34:], 16)
	copy(buffer[36:], "data")
	binary.LittleEndian.PutUint32(buffer[40:], 8000)
	return buffer
}
func (p *ModelPing) Ping(ctx context.Context, model, kind string) map[string]any {
	started := time.Now()
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	result := func(ok bool, status int, message any) map[string]any {
		out := map[string]any{"ok": ok, "latencyMs": time.Since(started).Milliseconds(), "error": message}
		if status != 0 {
			out["status"] = status
		}
		return out
	}
	failure := func(err error) map[string]any {
		if ctx.Err() == context.DeadlineExceeded {
			return result(false, 0, fmt.Sprintf("Test timed out after %.0fs — provider too slow or unreachable. The model may not exist on this provider, or the route (proxy pool / relay) is slow. Increase MODEL_TEST_TIMEOUT_MS to allow more time.", timeout.Seconds()))
		}
		return result(false, 0, err.Error())
	}
	path := "/api/v1/chat/completions"
	body := map[string]any{"model": model, "max_tokens": 1024, "stream": false, "messages": []any{map[string]any{"role": "user", "content": "hi"}}}
	switch kind {
	case "embedding":
		path = "/api/v1/embeddings"
		body = map[string]any{"model": model, "input": "test"}
	case "image":
		path = "/api/v1/images/generations"
		body = map[string]any{"model": model, "prompt": "test"}
	case "stt":
		path = "/api/v1/audio/transcriptions"
	}
	encoded, _ := json.Marshal(body)
	var reader io.Reader = bytes.NewReader(encoded)
	contentType := "application/json"
	if kind == "stt" {
		buffer := &bytes.Buffer{}
		form := multipart.NewWriter(buffer)
		part, err := form.CreateFormFile("file", "test.wav")
		if err != nil {
			return failure(err)
		}
		part.Write(silentTestWAV())
		form.WriteField("model", model)
		form.Close()
		contentType = form.FormDataContentType()
		reader = buffer
	}
	request, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(p.BaseURL, "/")+path, reader)
	if err != nil {
		return failure(err)
	}
	if p.Headers != nil {
		headers, err := p.Headers(ctx)
		if err != nil {
			return failure(err)
		}
		request.Header = headers.Clone()
	}
	request.Header.Set("Content-Type", contentType)
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return failure(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		return failure(err)
	}
	var parsed map[string]any
	_ = json.Unmarshal(raw, &parsed)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		nested, _ := parsed["error"].(map[string]any)
		detail := firstNonEmpty(nested["message"], parsed["msg"], parsed["message"], parsed["error"], string(raw))
		text := ttsString(detail)
		limit := 240
		if kind != "embedding" && kind != "image" && kind != "stt" {
			limit = 500
		}
		if len(text) > limit {
			text = text[:limit]
		}
		message := fmt.Sprintf("HTTP %d", response.StatusCode)
		if text != "" {
			message += ": " + text
		}
		return result(false, response.StatusCode, message)
	}
	switch kind {
	case "embedding":
		data, _ := parsed["data"].([]any)
		valid := false
		if len(data) > 0 {
			entry, _ := data[0].(map[string]any)
			_, valid = entry["embedding"].([]any)
		}
		if !valid {
			return result(false, response.StatusCode, "Provider returned no embedding data")
		}
	case "image":
		data, _ := parsed["data"].([]any)
		if len(data) == 0 {
			return result(false, response.StatusCode, "Provider returned no image data for this model")
		}
	case "stt":
		if strings.TrimSpace(ttsString(parsed["text"])) == "" {
			return result(false, response.StatusCode, "Provider returned no transcription text for this model")
		}
	default:
		if status := parsed["status"]; status != nil && jsString(status) != "200" && jsString(status) != "0" && nodeTruthy(firstNonEmpty(parsed["msg"], parsed["message"])) {
			return result(false, response.StatusCode, fmt.Sprintf("Provider status %v: %s", status, ttsString(firstNonEmpty(parsed["msg"], parsed["message"]))))
		}
		if nodeTruthy(parsed["error"]) {
			nested, _ := parsed["error"].(map[string]any)
			return result(false, response.StatusCode, ttsString(firstNonEmpty(nested["message"], parsed["error"], "Provider returned an error")))
		}
		choices, _ := parsed["choices"].([]any)
		if len(choices) == 0 {
			return result(false, response.StatusCode, "Provider returned no completion choices for this model")
		}
		first, _ := choices[0].(map[string]any)
		message, _ := first["message"].(map[string]any)
		if first["finish_reason"] == "length" && strings.TrimSpace(ttsString(message["content"])) == "" && nodeTruthy(firstNonEmpty(message["reasoning"], message["reasoning_content"], message["thinking"], message["thinking_content"])) {
			out := result(true, response.StatusCode, nil)
			out["note"] = "reasoning-only response (length-limited)"
			return out
		}
	}
	return result(true, response.StatusCode, nil)
}
func (p *ModelPing) Post(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		nodeReply(w, 500, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if !nodeTruthy(body["model"]) {
		nodeReply(w, 400, map[string]any{"error": "Model required"})
		return
	}
	kind := ttsString(body["kind"])
	if kind == "" {
		kind = "llm"
	}
	nodeReply(w, 200, p.Ping(r.Context(), ttsString(body["model"]), kind))
}

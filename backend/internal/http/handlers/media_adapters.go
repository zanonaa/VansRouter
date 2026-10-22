package handlers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func mediaAuth(config *providers.MediaConfig, token string) (string, string) {
	switch config.AuthHeader {
	case "token":
		return "Authorization", "Token " + token
	case "raw":
		return "Authorization", token
	case "x-api-key":
		return "x-api-key", token
	default:
		return "Authorization", "Bearer " + token
	}
}

func (g *Gateway) mediaConnection(w http.ResponseWriter, r *http.Request, provider string, config *providers.MediaConfig) (map[string]any, string, bool) {
	if config == nil {
		g.webError(w, 400, "Provider '"+provider+"' does not support this media operation")
		return nil, "", false
	}
	connection, err := g.connection(r.Context(), provider)
	token := ""
	if err == nil {
		token = ttsString(firstNonEmpty(connection["apiKey"], connection["accessToken"]))
	}
	if config.AuthType != "none" && token == "" {
		g.webError(w, 400, "No credentials for provider: "+provider)
		return nil, "", false
	}
	return connection, token, true
}

func (g *Gateway) Speech(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		g.webError(w, 400, "Invalid JSON body")
		return
	}
	key, ok := g.modelAuth(w, r)
	if !ok {
		return
	}
	if !gatewayPermission(key, "allowedKinds", "tts") {
		g.webError(w, 403, "TTS requests are not allowed for this API key")
		return
	}
	provider, model := splitTranslatorModel(ttsString(body["model"]))
	if provider == "" || ttsString(body["input"]) == "" {
		g.webError(w, 400, "Missing model or input")
		return
	}
	if !gatewayPermission(key, "allowedProviders", provider) {
		g.webError(w, 403, `Provider "`+provider+`" is not allowed for this API key`)
		return
	}
	config := providers.MediaConfigs()[provider].TTS
	_, token, ok := g.mediaConnection(w, r, provider, config)
	if !ok {
		return
	}
	if config.Format == "google-tts" {
		g.synthesizeGoogle(w, r, model, body)
		return
	}
	if config.Format == "gemini-tts" {
		g.synthesizeGemini(w, r, model, token, config, body)
		return
	}
	g.synthesizeNative(w, r, provider, model, token, config, body)
}

func (g *Gateway) Transcriptions(w http.ResponseWriter, r *http.Request) {
	key, ok := g.modelAuth(w, r)
	if !ok {
		return
	}
	if !gatewayPermission(key, "allowedKinds", "stt") {
		g.webError(w, 403, "STT requests are not allowed for this API key")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
		g.webError(w, 400, "Invalid multipart form data")
		return
	}
	defer r.MultipartForm.RemoveAll()
	provider, model := splitTranslatorModel(r.FormValue("model"))
	file, header, err := r.FormFile("file")
	if provider == "" || model == "" || err != nil {
		g.webError(w, 400, "Missing model or file")
		return
	}
	defer file.Close()
	if !gatewayPermission(key, "allowedProviders", provider) {
		g.webError(w, 403, `Provider "`+provider+`" is not allowed for this API key`)
		return
	}
	config := providers.MediaConfigs()[provider].STT
	_, token, ok := g.mediaConnection(w, r, provider, config)
	if !ok {
		return
	}
	switch config.Format {
	case "openai", "openai-whisper", "groq":
		var payload bytes.Buffer
		form := multipart.NewWriter(&payload)
		part, err := form.CreateFormFile("file", header.Filename)
		if err != nil {
			g.webError(w, 500, err.Error())
			return
		}
		if _, err = io.Copy(part, file); err != nil {
			g.webError(w, 400, err.Error())
			return
		}
		form.WriteField("model", model)
		for _, name := range []string{"language", "prompt", "response_format", "temperature"} {
			if value := r.FormValue(name); value != "" {
				form.WriteField(name, value)
			}
		}
		form.Close()
		request, err := http.NewRequestWithContext(r.Context(), "POST", config.BaseURL, &payload)
		if err != nil {
			g.webError(w, 500, err.Error())
			return
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", form.FormDataContentType())
		g.copyMedia(w, request)
	case "deepgram":
		values := url.Values{"model": {model}, "smart_format": {"true"}, "punctuate": {"true"}}
		if language := r.FormValue("language"); language != "" {
			values.Set("language", language)
		} else {
			values.Set("detect_language", "true")
		}
		request, err := http.NewRequestWithContext(r.Context(), "POST", config.BaseURL+"?"+values.Encode(), file)
		if err != nil {
			g.webError(w, 500, err.Error())
			return
		}
		name, value := mediaAuth(config, token)
		request.Header.Set(name, value)
		request.Header.Set("Content-Type", audioContentType(header))
		g.transcriptionResult(w, request, "deepgram")
	case "assemblyai":
		g.transcribeAssemblyAI(w, r, config, file, model, token, waitTranscriptionPoll)
	case "gemini-stt":
		g.transcribeGemini(w, r, config, file, header, model, token)
	case "huggingface-asr":
		if strings.Contains(model, "..") || strings.Contains(model, "//") {
			g.webError(w, 400, "Invalid model ID")
			return
		}
		request, err := http.NewRequestWithContext(r.Context(), "POST", strings.TrimRight(config.BaseURL, "/")+"/"+model, file)
		if err != nil {
			g.webError(w, 500, err.Error())
			return
		}
		name, value := mediaAuth(config, token)
		request.Header.Set(name, value)
		request.Header.Set("Content-Type", audioContentType(header))
		g.transcriptionResult(w, request, "huggingface-asr")
	default:
		g.webError(w, 501, "STT adapter not implemented: "+config.Format)
	}
}

func (g *Gateway) copyMedia(w http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 300*time.Second)
	defer cancel()
	request = request.WithContext(ctx)
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		status := 502
		if ctx.Err() == context.DeadlineExceeded {
			status = 504
		}
		g.webError(w, status, err.Error())
		return
	}
	defer response.Body.Close()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
}

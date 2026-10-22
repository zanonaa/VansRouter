package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ProviderValidation executa as sondas reais de chave. Provedores sem contrato
// local retornam erro explícito, nunca valid=true simulado.
type ProviderValidation struct {
	Client *http.Client
	Nodes  NodeStore
}

func (p *ProviderValidation) do(ctx context.Context, method, url string, headers map[string]string, body []byte) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, err
	}
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	client := p.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	response.Body.Close()
	return response.StatusCode, nil
}
func (p *ProviderValidation) Post(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		nodeReply(w, 400, map[string]any{"error": "Invalid JSON body"})
		return
	}
	provider := providersNormalize(ttsString(body["provider"]))
	key := ttsString(body["apiKey"])
	data, _ := body["providerSpecificData"].(map[string]any)
	if provider == "" || key == "" && provider != "ollama-local" {
		nodeReply(w, 400, map[string]any{"error": "Provider and API key required"})
		return
	}
	status, probeErr := 0, error(nil)
	switch provider {
	case "openai":
		status, probeErr = p.do(r.Context(), "GET", "https://api.openai.com/v1/models", map[string]string{"Authorization": "Bearer " + key}, nil)
	case "anthropic":
		status, probeErr = p.do(r.Context(), "POST", "https://api.anthropic.com/v1/messages", map[string]string{"x-api-key": key, "anthropic-version": "2023-06-01", "content-type": "application/json"}, []byte(`{"model":"claude-3-haiku-20240307","max_tokens":1,"messages":[{"role":"user","content":"test"}]}`))
	case "gemini":
		status, probeErr = p.do(r.Context(), "GET", "https://generativelanguage.googleapis.com/v1/models?key="+key, nil, nil)
	default:
		nodeReply(w, 501, map[string]any{"error": fmt.Sprintf("Provider validation not ported: %s", provider), "valid": false})
		return
	}
	if probeErr != nil {
		nodeReply(w, 502, map[string]any{"error": probeErr.Error(), "valid": false})
		return
	}
	valid := status >= 200 && status < 300
	if provider == "anthropic" {
		valid = status != 401
	}
	errorText := any(nil)
	if !valid {
		errorText = "Invalid API key"
	}
	nodeReply(w, 200, map[string]any{"valid": valid, "error": errorText, "providerSpecificData": data})
}
func providersNormalize(value string) string { return strings.TrimSpace(strings.ToLower(value)) }

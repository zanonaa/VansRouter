package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Device flows: qwen (PKCE), kimi/kilocode/glm (sem PKCE) com form padrão.
func (h *OAuthEngine) deviceCodeRequest(ctx context.Context, provider string, form url.Values) (map[string]any, error) {
	config := h.config(provider)
	if config == nil || config.DeviceCodeURL == "" {
		return nil, errDeviceNotSupported(provider)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", config.DeviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := h.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, failMessage("Device code request failed", raw)
	}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return nil, failMessage("Device code request failed", raw)
	}
	return data, nil
}
func (h *OAuthEngine) devicePoll(ctx context.Context, provider, deviceCode, codeVerifier string) (bool, map[string]any, error) {
	config := h.config(provider)
	if config == nil || config.TokenURL == "" {
		return false, nil, errDeviceNotSupported(provider)
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "client_id": {config.ClientID}, "device_code": {deviceCode}}
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return false, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := h.client().Do(req)
	if err != nil {
		return false, nil, err
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	if data == nil {
		data = map[string]any{"error": "invalid_response"}
	}
	return res.StatusCode >= 200 && res.StatusCode < 300, data, nil
}

var devicePKCEProviders = map[string]bool{"qwen": true, "qoder": true}

func (h *OAuthEngine) genericDeviceCode(w http.ResponseWriter, r *http.Request, provider string) {
	config := h.config(provider)
	if config == nil || config.DeviceCodeURL == "" {
		nodeReply(w, 400, map[string]string{"error": "Provider " + provider + " does not support device code flow"})
		return
	}
	form := url.Values{"client_id": {config.ClientID}}
	if config.DeviceScope != "" {
		form.Set("scope", config.DeviceScope)
	}
	verifier, challenge, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to generate PKCE"})
		return
	}
	if devicePKCEProviders[provider] && config.CodeChallengeMethod != "" {
		form.Set("code_challenge", challenge)
		form.Set("code_challenge_method", config.CodeChallengeMethod)
	}
	data, err := h.deviceCodeRequest(r.Context(), provider, form)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	data["codeVerifier"] = verifier
	nodeReply(w, 200, data)
}
func (h *OAuthEngine) genericDevicePoll(w http.ResponseWriter, r *http.Request, provider string, requireVerifier bool, mapTokens func(map[string]any) map[string]any) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid or empty request body"})
		return
	}
	deviceCode, _ := body["deviceCode"].(string)
	codeVerifier, _ := body["codeVerifier"].(string)
	if deviceCode == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing device code"})
		return
	}
	if requireVerifier && codeVerifier == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing code verifier"})
		return
	}
	ok, data, err := h.devicePoll(r.Context(), provider, deviceCode, codeVerifier)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	access, _ := data["access_token"].(string)
	if !ok || access == "" {
		result := map[string]any{"success": false}
		if v, exists := data["error"]; exists {
			result["error"] = v
		}
		if v, exists := data["error_description"]; exists {
			result["errorDescription"] = v
		}
		if ok {
			errorCode, _ := data["error"].(string)
			if errorCode == "authorization_pending" || errorCode == "slow_down" {
				result["pending"] = errorCode == "authorization_pending"
			} else if !nodeTruthy(result["error"]) {
				result["error"] = "no_access_token"
			}
		}
		nodeReply(w, 200, result)
		return
	}
	mapped := mapTokens(data)
	mapped["provider"] = provider
	mapped["authType"] = "oauth"
	mapped["testStatus"] = "active"
	if seconds, okSeconds := data["expires_in"].(float64); okSeconds {
		mapped["expiresIn"] = seconds
		mapped["expiresAt"] = time.Now().Add(time.Duration(seconds) * time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
	}
	connection, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": connection["id"], "provider": connection["provider"]}})
}
func errDeviceNotSupported(provider string) error {
	return failText("Provider " + provider + " does not support device code flow")
}

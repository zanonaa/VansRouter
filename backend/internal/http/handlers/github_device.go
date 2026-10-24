package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (h *OAuthEngine) githubRequest(ctx context.Context, target string, form url.Values, token string) (int, map[string]any, error) {
	method := "GET"
	var body io.Reader
	if form != nil {
		method = "POST"
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if token != "" {
		config := h.config("github")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-GitHub-Api-Version", config.APIVersion)
		req.Header.Set("User-Agent", config.UserAgent)
	}
	res, err := h.client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return 0, nil, err
	}
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		data = map[string]any{"error": "invalid_response", "error_description": string(raw)}
	}
	return res.StatusCode, data, nil
}
func (h *OAuthEngine) githubDevice(w http.ResponseWriter, r *http.Request) {
	c := h.config("github")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: github"})
		return
	}
	status, data, err := h.githubRequest(r.Context(), c.DeviceCodeURL, url.Values{"client_id": {c.ClientID}, "scope": {c.DeviceScope}}, "")
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Device code request failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 500, map[string]string{"error": fmt.Sprintf("Device code request failed (%d)", status)})
		return
	}
	verifier, _, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to generate PKCE"})
		return
	}
	data["codeVerifier"] = verifier
	nodeReply(w, 200, data)
}
func (h *OAuthEngine) githubPoll(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid or empty request body"})
		return
	}
	code := body["deviceCode"]
	if !nodeTruthy(code) {
		nodeReply(w, 400, map[string]string{"error": "Missing device code"})
		return
	}
	c := h.config("github")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: github"})
		return
	}
	status, data, err := h.githubRequest(r.Context(), c.TokenURL, url.Values{"client_id": {c.ClientID}, "device_code": {jsString(code)}, "grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}}, "")
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Token poll failed"})
		return
	}
	access, _ := data["access_token"].(string)
	if status < 200 || status >= 300 || access == "" {
		result := map[string]any{"success": false}
		if value, exists := data["error"]; exists {
			result["error"] = value
		}
		if value, exists := data["error_description"]; exists {
			result["errorDescription"] = value
		}
		if status >= 200 && status < 300 {
			errorCode, _ := data["error"].(string)
			if errorCode == "authorization_pending" || errorCode == "slow_down" {
				result["pending"] = errorCode == "authorization_pending"
				if _, exists := result["errorDescription"]; !exists {
					if value, ok := data["message"]; ok {
						result["errorDescription"] = value
					}
				}
			} else {
				if !nodeTruthy(result["error"]) {
					result["error"] = "no_access_token"
				}
				if !nodeTruthy(result["errorDescription"]) {
					result["errorDescription"] = legacyMessage(data)
				}
			}
		}
		nodeReply(w, 200, result)
		return
	}
	copilotStatus, copilot, err := h.githubRequest(r.Context(), c.CopilotTokenURL, nil, access)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Copilot token request failed"})
		return
	}
	if copilotStatus < 200 || copilotStatus >= 300 {
		copilot = map[string]any{}
	}
	userStatus, user, err := h.githubRequest(r.Context(), c.UserInfoURL, nil, access)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "GitHub profile request failed"})
		return
	}
	if userStatus < 200 || userStatus >= 300 {
		user = map[string]any{}
	}
	mapped := map[string]any{"provider": "github", "authType": "oauth", "accessToken": access, "email": nil, "testStatus": "active", "expiresAt": nil}
	for source, target := range map[string]string{"refresh_token": "refreshToken", "expires_in": "expiresIn"} {
		if value, exists := data[source]; exists {
			mapped[target] = value
		}
	}
	if seconds, ok := data["expires_in"].(float64); ok && seconds != 0 {
		mapped["expiresAt"] = time.Now().Add(time.Duration(seconds) * time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
	}
	name := user["login"]
	if !nodeTruthy(name) {
		name = user["name"]
	}
	if name != nil {
		mapped["name"] = name
	}
	display := user["name"]
	if !nodeTruthy(display) {
		display = user["login"]
	}
	if display != nil {
		mapped["displayName"] = display
	}
	if nodeTruthy(user["email"]) {
		mapped["email"] = user["email"]
	}
	specific := map[string]any{}
	for source, target := range map[string]string{"token": "copilotToken", "expires_at": "copilotTokenExpiresAt"} {
		if value, exists := copilot[source]; exists {
			specific[target] = value
		}
	}
	for source, target := range map[string]string{"id": "githubUserId", "login": "githubLogin", "name": "githubName", "email": "githubEmail"} {
		if value, exists := user[source]; exists {
			specific[target] = value
		}
	}
	mapped["providerSpecificData"] = specific
	connection, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": connection["id"], "provider": connection["provider"]}})
}
func legacyMessage(data map[string]any) any {
	if nodeTruthy(data["message"]) {
		return data["message"]
	}
	return "No access token received"
}

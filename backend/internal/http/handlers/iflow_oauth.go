package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// iFlow: authorization_code com Basic Auth + user info obrigatória e apiKey.
func (h *OAuthEngine) iflowAuthorize(w http.ResponseWriter, r *http.Request, redirectURI string) {
	config := h.config("iflow")
	if config == nil || config.AuthorizeURL == "" {
		nodeReply(w, 501, map[string]string{"error": "OAuth authorization not yet migrated for: iflow"})
		return
	}
	verifier, challenge, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to generate PKCE"})
		return
	}
	state, err := randomURLSafe(32)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to generate state"})
		return
	}
	loginMethod, loginType := "", ""
	for key, value := range config.ExtraParams {
		if key == "loginMethod" {
			loginMethod = value
		}
		if key == "type" {
			loginType = value
		}
	}
	params := url.Values{"loginMethod": {loginMethod}, "type": {loginType}, "redirect": {redirectURI}, "state": {state}, "client_id": {config.ClientID}}
	nodeReply(w, 200, map[string]any{"authUrl": config.AuthorizeURL + "?" + params.Encode(), "state": state, "codeVerifier": verifier, "codeChallenge": challenge, "redirectUri": redirectURI, "flowType": "authorization_code", "callbackPath": "/callback"})
}
func (h *OAuthEngine) iflowExchange(w http.ResponseWriter, r *http.Request, body map[string]any) {
	code, _ := body["code"].(string)
	redirectURI, _ := body["redirectUri"].(string)
	if code == "" || redirectURI == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing required fields"})
		return
	}
	config := h.config("iflow")
	if config == nil || config.TokenURL == "" {
		nodeReply(w, 501, map[string]string{"error": "Provider token exchange not yet migrated for: iflow"})
		return
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI}, "client_id": {config.ClientID}, "client_secret": {config.ClientSecret}}
	req, err := http.NewRequestWithContext(r.Context(), "POST", config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Token exchange failed"})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Basic "+base64Std(config.ClientID+":"+config.ClientSecret))
	res, err := h.client().Do(req)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Token exchange failed"})
		return
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var tokens map[string]any
	if json.Unmarshal(raw, &tokens) != nil {
		tokens = map[string]any{}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		nodeReply(w, 500, map[string]string{"error": fmt.Sprintf("Token exchange failed: %s", string(raw))})
		return
	}
	access, _ := tokens["access_token"].(string)
	userURL := config.UserInfoURL
	if userURL == "" {
		nodeReply(w, 500, map[string]string{"error": "iFlow user info URL missing"})
		return
	}
	userReq, err := http.NewRequestWithContext(r.Context(), "GET", userURL+"?accessToken="+url.QueryEscape(access), nil)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch user info"})
		return
	}
	userReq.Header.Set("Accept", "application/json")
	userRes, err := h.client().Do(userReq)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch user info"})
		return
	}
	defer userRes.Body.Close()
	userRaw, _ := io.ReadAll(io.LimitReader(userRes.Body, 1<<20))
	if userRes.StatusCode < 200 || userRes.StatusCode >= 300 {
		nodeReply(w, 500, map[string]string{"error": fmt.Sprintf("Failed to fetch user info: %s", string(userRaw))})
		return
	}
	var envelope struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if json.Unmarshal(userRaw, &envelope) != nil || !envelope.Success {
		message := envelope.Message
		if message == "" {
			message = "Unknown error"
		}
		nodeReply(w, 500, map[string]string{"error": "User info request failed: " + message})
		return
	}
	info := envelope.Data
	apiKey, _ := info["apiKey"].(string)
	if strings.TrimSpace(apiKey) == "" {
		nodeReply(w, 500, map[string]string{"error": "Empty API key returned from iFlow"})
		return
	}
	email := firstNonEmptyString(info["email"], info["phone"])
	if email == "" {
		nodeReply(w, 500, map[string]string{"error": "Missing account email/phone in user info"})
		return
	}
	data := map[string]any{"provider": "iflow", "authType": "oauth", "accessToken": access, "apiKey": apiKey, "email": email, "displayName": firstNonEmpty(info["nickname"], info["name"]), "testStatus": "active"}
	if value, exists := tokens["refresh_token"]; exists {
		data["refreshToken"] = value
	}
	if seconds, ok := tokens["expires_in"].(float64); ok {
		data["expiresIn"] = seconds
		data["expiresAt"] = time.Now().Add(time.Duration(seconds) * time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
	}
	connection, err := h.CreateConnection(r.Context(), data)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": connection["id"], "provider": connection["provider"], "email": connection["email"], "displayName": connection["displayName"]}})
}
func firstNonEmptyString(values ...any) string {
	for _, v := range values {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
func firstNonEmpty(values ...any) any {
	for _, v := range values {
		if nodeTruthy(v) {
			return v
		}
	}
	return nil
}
func base64Std(value string) string { return encodeBase64Std(value) }

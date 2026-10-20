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

// postExchange de gemini-cli: userInfo + projectId, ambos best-effort.
func (h *OAuthEngine) geminiPost(ctx context.Context, accessToken string) (map[string]any, string) {
	userInfo := map[string]any{}
	projectId := ""
	if target := h.config("gemini-cli"); target != nil && target.UserInfoURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", target.UserInfoURL+"?alt=json", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+accessToken)
			if res, err := h.client().Do(req); err == nil {
				if res.StatusCode >= 200 && res.StatusCode < 300 {
					_ = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&userInfo)
				}
				res.Body.Close()
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", strings.NewReader(`{"metadata":{"ideType":9,"platform":0,"pluginType":2},"mode":1}`))
	if err == nil {
		req.Header.Set("Authorization", "Bearer "+accessToken)
		req.Header.Set("Content-Type", "application/json")
		if res, err := h.client().Do(req); err == nil {
			if res.StatusCode >= 200 && res.StatusCode < 300 {
				var data struct {
					CloudaicompanionProject any `json:"cloudaicompanionProject"`
				}
				if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&data) == nil {
					switch project := data.CloudaicompanionProject.(type) {
					case string:
						projectId = project
					case map[string]any:
						if id, ok := project["id"].(string); ok {
							projectId = id
						}
					}
				}
			}
			res.Body.Close()
		}
	}
	return userInfo, projectId
}
func (h *OAuthEngine) geminiAuthorize(w http.ResponseWriter, r *http.Request, redirectURI string) {
	config := h.config("gemini-cli")
	if config == nil || config.AuthorizeURL == "" {
		nodeReply(w, 501, map[string]string{"error": "OAuth authorization not yet migrated for: gemini-cli"})
		return
	}
	verifier, _, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to generate PKCE"})
		return
	}
	state, err := randomURLSafe(32)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to generate state"})
		return
	}
	params := url.Values{"client_id": {config.ClientID}, "response_type": {"code"}, "redirect_uri": {redirectURI}, "scope": {strings.Join(config.Scopes, " ")}, "state": {state}, "access_type": {"offline"}, "prompt": {"consent"}}
	nodeReply(w, 200, map[string]any{"authUrl": config.AuthorizeURL + "?" + params.Encode(), "state": state, "codeVerifier": verifier, "redirectUri": redirectURI, "flowType": "authorization_code", "callbackPath": "/callback"})
}
func (h *OAuthEngine) geminiExchange(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid or empty request body"})
		return
	}
	code, _ := body["code"].(string)
	redirectURI, _ := body["redirectUri"].(string)
	if code == "" || redirectURI == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing required fields"})
		return
	}
	config := h.config("gemini-cli")
	if config == nil || config.TokenURL == "" {
		nodeReply(w, 501, map[string]string{"error": "Provider token exchange not yet migrated for: gemini-cli"})
		return
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {config.ClientID}, "client_secret": {config.ClientSecret}, "code": {code}, "redirect_uri": {redirectURI}}
	req, err := http.NewRequestWithContext(r.Context(), "POST", config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Token exchange failed"})
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
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
	userInfo, projectId := h.geminiPost(r.Context(), access)
	data := map[string]any{"provider": "gemini-cli", "authType": "oauth", "accessToken": access, "email": userInfo["email"], "projectId": projectId, "testStatus": "active"}
	if value, exists := tokens["refresh_token"]; exists {
		data["refreshToken"] = value
	}
	if value, exists := tokens["scope"]; exists {
		data["scope"] = value
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

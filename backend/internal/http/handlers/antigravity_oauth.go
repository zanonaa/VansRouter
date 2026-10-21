package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"
)

func (h *OAuthEngine) antigravityAuthorize(w http.ResponseWriter, r *http.Request, redirectURI string) {
	c := h.config("antigravity")
	if c == nil || c.AuthorizeURL == "" {
		nodeReply(w, 500, map[string]string{"error": "Unknown OAuth provider: antigravity"})
		return
	}
	verifier, challenge, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	state, err := randomURLSafe(32)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	params := url.Values{"client_id": {c.ClientID}, "response_type": {"code"}, "redirect_uri": {redirectURI}, "scope": {strings.Join(c.Scopes, " ")}, "state": {state}, "access_type": {"offline"}, "prompt": {"consent"}}
	nodeReply(w, 200, map[string]any{"authUrl": c.AuthorizeURL + "?" + params.Encode(), "state": state, "codeVerifier": verifier, "codeChallenge": challenge, "redirectUri": redirectURI, "flowType": "authorization_code", "callbackPath": "/callback"})
}

func (h *OAuthEngine) antigravityPost(ctx context.Context, access, userInfoURL, loadURL string) (map[string]any, string, string) {
	userInfo := map[string]any{}
	projectId, tierId := "", "legacy-tier"
	if userInfoURL != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", userInfoURL+"?alt=json", nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+access)
			req.Header.Set("x-request-source", "local")
			if c := h.config("antigravity"); c != nil && c.LoadCodeAssistUserAgent != "" {
				req.Header.Set("User-Agent", c.LoadCodeAssistUserAgent)
			}
			if res, err := h.client().Do(req); err == nil {
				if res.StatusCode >= 200 && res.StatusCode < 300 {
					_ = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&userInfo)
				}
				res.Body.Close()
			}
		}
	}
	if loadURL != "" {
		platform := 0
		switch {
		case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
			platform = 2
		case runtime.GOOS == "darwin":
			platform = 1
		case runtime.GOOS == "linux" && runtime.GOARCH == "arm64":
			platform = 4
		case runtime.GOOS == "linux":
			platform = 3
		case runtime.GOOS == "windows":
			platform = 5
		}
		payload, _ := json.Marshal(map[string]any{"metadata": map[string]int{"ideType": 9, "platform": platform, "pluginType": 2}})
		req, err := http.NewRequestWithContext(ctx, "POST", loadURL, strings.NewReader(string(payload)))
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+access)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("x-request-source", "local")
			if c := h.config("antigravity"); c != nil && c.LoadCodeAssistUserAgent != "" {
				req.Header.Set("User-Agent", c.LoadCodeAssistUserAgent)
			}
			if res, err := h.client().Do(req); err == nil {
				if res.StatusCode >= 200 && res.StatusCode < 300 {
					var data struct {
						CloudaicompanionProject any `json:"cloudaicompanionProject"`
						AllowedTiers            []struct {
							IsDefault bool   `json:"isDefault"`
							ID        string `json:"id"`
						} `json:"allowedTiers"`
					}
					if json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&data) == nil {
						switch project := data.CloudaicompanionProject.(type) {
						case string:
							projectId = project
						case map[string]any:
							projectId, _ = project["id"].(string)
						}
						for _, tier := range data.AllowedTiers {
							if tier.IsDefault && tier.ID != "" {
								tierId = strings.TrimSpace(tier.ID)
								break
							}
						}
					}
				}
				res.Body.Close()
			}
		}
	}
	return userInfo, projectId, tierId
}

func (h *OAuthEngine) antigravityExchange(w http.ResponseWriter, r *http.Request, body map[string]any) {
	code, _ := body["code"].(string)
	redirectURI, _ := body["redirectUri"].(string)
	if code == "" || redirectURI == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing required fields"})
		return
	}
	c := h.config("antigravity")
	if c == nil || c.TokenURL == "" {
		nodeReply(w, 500, map[string]string{"error": "Unknown OAuth provider: antigravity"})
		return
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {c.ClientID}, "client_secret": {c.ClientSecret}, "code": {code}, "redirect_uri": {redirectURI}}
	status, data, raw, err := h.deviceJSON(r.Context(), "POST", c.TokenURL, map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json"}, strings.NewReader(form.Encode()))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Token exchange failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 500, map[string]string{"error": fmt.Sprintf("Token exchange failed: %s", string(raw))})
		return
	}
	access := stringField(data, "access_token")
	loadURL := c.LoadCodeAssistEndpoint
	if loadURL == "" {
		loadURL = c.ProjectURL
	}
	userInfo, projectId, tier := h.antigravityPost(r.Context(), access, c.UserInfoURL, loadURL)
	if projectId != "" && c.OnboardUserEndpoint != "" {
		go h.antigravityOnboard(access, tier, c.OnboardUserEndpoint, c.LoadCodeAssistUserAgent)
	}
	mapped := map[string]any{"provider": "antigravity", "authType": "oauth", "accessToken": access, "email": userInfo["email"], "projectId": projectId, "testStatus": "active"}
	if value, ok := data["refresh_token"]; ok {
		mapped["refreshToken"] = value
	}
	if value, ok := data["scope"]; ok {
		mapped["scope"] = value
	}
	if seconds, ok := data["expires_in"].(float64); ok {
		mapped["expiresIn"] = seconds
		mapped["expiresAt"] = time.Now().Add(time.Duration(seconds) * time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
	}
	saved, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"], "email": saved["email"], "displayName": saved["displayName"]}})
}

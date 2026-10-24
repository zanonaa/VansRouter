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

func (h *OAuthEngine) gitlabAuthorize(w http.ResponseWriter, r *http.Request, redirectURI string) {
	c := h.config("gitlab")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown OAuth provider: gitlab"})
		return
	}
	meta := r.URL.Query()
	baseUrl := meta.Get("baseUrl")
	if baseUrl == "" {
		baseUrl = c.DefaultBaseURL
		if baseUrl == "" {
			baseUrl = "https://gitlab.com"
		}
	}
	clientId := meta.Get("clientId")
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
	path := c.AuthorizeURLPath
	if path == "" {
		path = "/oauth/authorize"
	}
	params := url.Values{"client_id": {clientId}, "redirect_uri": {redirectURI}, "response_type": {"code"}, "state": {state}, "scope": {c.Scope}, "code_challenge": {challenge}, "code_challenge_method": {c.CodeChallengeMethod}}
	nodeReply(w, 200, map[string]any{"authUrl": baseUrl + path + "?" + params.Encode(), "state": state, "codeVerifier": verifier, "codeChallenge": challenge, "redirectUri": redirectURI, "flowType": "authorization_code_pkce", "callbackPath": "/callback"})
}

func (h *OAuthEngine) gitlabExchange(w http.ResponseWriter, r *http.Request, body map[string]any) {
	code, _ := body["code"].(string)
	redirectURI, _ := body["redirectUri"].(string)
	codeVerifier, _ := body["codeVerifier"].(string)
	if code == "" || redirectURI == "" || codeVerifier == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing required fields"})
		return
	}
	meta, _ := body["meta"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
	}
	c := h.config("gitlab")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown OAuth provider: gitlab"})
		return
	}
	baseUrl, _ := meta["baseUrl"].(string)
	if baseUrl == "" {
		baseUrl = c.DefaultBaseURL
		if baseUrl == "" {
			baseUrl = "https://gitlab.com"
		}
	}
	clientId, _ := meta["clientId"].(string)
	clientSecret, _ := meta["clientSecret"].(string)
	tokenPath := c.TokenURLPath
	if tokenPath == "" {
		tokenPath = "/oauth/token"
	}
	form := url.Values{"client_id": {clientId}, "grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirectURI}, "code_verifier": {codeVerifier}}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	status, tokens, raw, err := h.deviceJSON(r.Context(), "POST", baseUrl+tokenPath, map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json"}, strings.NewReader(form.Encode()))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "GitLab token exchange failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 500, map[string]string{"error": fmt.Sprintf("GitLab token exchange failed: %s", string(raw))})
		return
	}
	// User info
	userPath := c.UserInfoURLPath
	if userPath == "" {
		userPath = "/api/v4/user"
	}
	userReq, err := http.NewRequestWithContext(r.Context(), "GET", baseUrl+userPath, nil)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "GitLab user info failed"})
		return
	}
	userReq.Header.Set("Authorization", "Bearer "+stringField(tokens, "access_token"))
	userRes, err := h.client().Do(userReq)
	user := map[string]any{}
	if err == nil {
		defer userRes.Body.Close()
		if userRes.StatusCode >= 200 && userRes.StatusCode < 300 {
			raw, _ := io.ReadAll(io.LimitReader(userRes.Body, 1<<20))
			_ = json.Unmarshal(raw, &user)
		}
	}
	specific := map[string]any{
		"username": firstNonEmptyString(user["username"]),
		"email":    firstNonEmptyString(user["email"], user["public_email"]),
		"name":     firstNonEmptyString(user["name"]),
		"baseUrl":  baseUrl,
		"clientId": clientId,
		"authKind": "oauth",
	}
	mapped := map[string]any{"provider": "gitlab", "authType": "oauth", "testStatus": "active", "providerSpecificData": specific}
	for source, target := range map[string]string{"access_token": "accessToken", "refresh_token": "refreshToken", "scope": "scope"} {
		if v, ok := tokens[source]; ok {
			mapped[target] = v
		}
	}
	if seconds, ok := tokens["expires_in"].(float64); ok {
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

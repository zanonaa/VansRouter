package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func (h *OAuthEngine) kimchiAuthorize(w http.ResponseWriter, r *http.Request, redirectURI string) {
	c := h.config("kimchi")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown OAuth provider: kimchi"})
		return
	}
	base := c.WebAppURL
	if base == "" {
		base = "https://app.kimchi.dev"
	}
	base = strings.TrimRight(base, "/")
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
	params := url.Values{"callback": {redirectURI}, "state": {state}}
	nodeReply(w, 200, map[string]any{"authUrl": base + "/cli-auth?" + params.Encode(), "state": state, "codeVerifier": verifier, "codeChallenge": challenge, "redirectUri": redirectURI, "flowType": "browser_token", "callbackPath": "/callback"})
}
func (h *OAuthEngine) kimchiExchange(w http.ResponseWriter, r *http.Request, body map[string]any) {
	token := strings.TrimSpace(stringField(body, "code"))
	if token == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing Kimchi token"})
		return
	}
	c := h.config("kimchi")
	if c == nil || c.ValidationURL == "" {
		nodeReply(w, 500, map[string]string{"error": "Kimchi validation URL is not configured"})
		return
	}
	fetchJSON := func(target string) (int, map[string]any, error) {
		req, err := http.NewRequestWithContext(r.Context(), "GET", target, nil)
		if err != nil {
			return 0, nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := h.client().Do(req)
		if err != nil {
			return 0, nil, err
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		var data map[string]any
		_ = json.Unmarshal(raw, &data)
		return res.StatusCode, data, nil
	}
	if status, _, err := fetchJSON(c.ValidationURL); err != nil {
		nodeReply(w, 500, map[string]string{"error": "Kimchi token validation failed"})
		return
	} else if status < 200 || status >= 300 {
		nodeReply(w, 500, map[string]string{"error": fmt.Sprintf("Kimchi token validation failed: %d", status)})
		return
	}
	user := map[string]any{}
	if c.UserInfoURL != "" {
		if status, data, err := fetchJSON(c.UserInfoURL); err == nil && status >= 200 && status < 300 && data != nil {
			user = data
		}
	}
	userId := ""
	if nodeTruthy(user["id"]) {
		userId = jsString(user["id"])
	}
	username := ""
	if nodeTruthy(user["username"]) {
		username = jsString(user["username"])
	}
	email := ""
	if nodeTruthy(user["email"]) {
		email = jsString(user["email"])
	}
	if email == "" && userId != "" {
		email = "kimchi-user-" + userId
	}
	var displayName any
	displayName = firstNonEmpty(user["name"], user["username"])
	specific := map[string]any{"authMethod": "browser_token", "userId": userId, "username": username}
	mapped := map[string]any{"provider": "kimchi", "authType": "oauth", "accessToken": token, "refreshToken": nil, "testStatus": "active", "expiresIn": nil, "expiresAt": nil, "providerSpecificData": specific}
	if email != "" {
		mapped["email"] = email
	} else {
		mapped["email"] = nil
	}
	if displayName != nil {
		mapped["displayName"] = displayName
	}
	saved, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	result := map[string]any{"id": saved["id"], "provider": saved["provider"]}
	for _, key := range []string{"email", "displayName"} {
		if value, ok := saved[key]; ok {
			result[key] = value
		}
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": result})
}

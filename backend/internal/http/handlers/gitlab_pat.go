package handlers

import (
	"net/http"
	"strings"
)

// GitlabPAT verifica o PAT na instância antes de persistir qualquer credencial.
func (h *OAuthEngine) GitlabPAT(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid request body"})
		return
	}
	if body == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid request body"})
		return
	}
	token, ok := body["token"].(string)
	if !ok && body["token"] != nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid token type"})
		return
	}
	token = strings.TrimSpace(token)
	if token == "" {
		nodeReply(w, 400, map[string]string{"error": "Personal Access Token is required"})
		return
	}
	base := "https://gitlab.com"
	if raw := body["baseUrl"]; raw != nil {
		value, ok := raw.(string)
		if !ok {
			nodeReply(w, 500, map[string]string{"error": "Invalid base URL type"})
			return
		}
		if strings.TrimSpace(value) != "" {
			base = strings.TrimSpace(value)
		}
	}
	base = strings.TrimSuffix(base, "/")
	status, user, raw, err := h.deviceJSON(r.Context(), "GET", base+"/api/v4/user", map[string]string{"Private-Token": token, "Accept": "application/json"}, nil)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "GitLab token verification failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 401, map[string]string{"error": "GitLab token verification failed: " + string(raw)})
		return
	}
	if user == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid GitLab user response"})
		return
	}
	email := firstNonEmpty(user["email"], user["public_email"])
	if email == nil {
		email = ""
	}
	username := user["username"]
	if !nodeTruthy(username) {
		username = ""
	}
	name := user["name"]
	if !nodeTruthy(name) {
		name = ""
	}
	data := map[string]any{"provider": "gitlab", "authType": "oauth", "accessToken": token, "refreshToken": nil, "expiresAt": nil, "email": email, "displayName": firstNonEmpty(name, username, email), "testStatus": "active", "providerSpecificData": map[string]any{"username": username, "email": email, "name": name, "baseUrl": base, "authKind": "personal_access_token"}}
	if _, err = h.CreateConnection(r.Context(), data); err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}

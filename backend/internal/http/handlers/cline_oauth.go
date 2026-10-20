package handlers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (h *OAuthEngine) clineAuthorize(w http.ResponseWriter, r *http.Request, provider, redirectURI string) {
	c := h.config(provider)
	if c == nil || c.AuthorizeURL == "" {
		nodeReply(w, 500, map[string]string{"error": "Unknown OAuth provider: " + provider})
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
	params := url.Values{"client_type": {"extension"}, "callback_url": {redirectURI}, "redirect_uri": {redirectURI}}
	nodeReply(w, 200, map[string]any{"authUrl": c.AuthorizeURL + "?" + params.Encode(), "state": state, "codeVerifier": verifier, "codeChallenge": challenge, "redirectUri": redirectURI, "flowType": "authorization_code", "callbackPath": "/callback"})
}
func clineDecoded(code string) (map[string]any, bool) {
	clean := strings.TrimRight(code, "=")
	raw, err := base64.RawStdEncoding.DecodeString(clean)
	if err != nil {
		return nil, false
	}
	end := strings.LastIndex(string(raw), "}")
	if end < 0 {
		return nil, false
	}
	var data map[string]any
	if json.Unmarshal(raw[:end+1], &data) != nil || data == nil {
		return nil, false
	}
	return data, true
}
func (h *OAuthEngine) clineExchange(w http.ResponseWriter, r *http.Request, provider string, body map[string]any) {
	code, _ := body["code"].(string)
	redirectURI, _ := body["redirectUri"].(string)
	c := h.config(provider)
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown OAuth provider: " + provider})
		return
	}
	decoded, inline := clineDecoded(code)
	tokens := map[string]any{}
	if inline {
		for source, target := range map[string]string{"accessToken": "accessToken", "refreshToken": "refreshToken", "email": "email", "firstName": "firstName", "lastName": "lastName", "expiresAt": "expires_at"} {
			if value, ok := decoded[source]; ok {
				tokens[target] = value
			}
		}
	} else {
		target := c.TokenExchangeURL
		if provider == "clinepass" {
			target = c.TokenURL
		}
		raw, _ := json.Marshal(map[string]string{"grant_type": "authorization_code", "code": code, "client_type": "extension", "redirect_uri": redirectURI})
		status, data, text, err := h.deviceJSON(r.Context(), "POST", target, map[string]string{"Content-Type": "application/json", "Accept": "application/json"}, strings.NewReader(string(raw)))
		if err != nil {
			nodeReply(w, 500, map[string]string{"error": "Token exchange failed"})
			return
		}
		label := "Cline"
		if provider == "clinepass" {
			label = "ClinePass"
		}
		if status < 200 || status >= 300 {
			nodeReply(w, 500, map[string]string{"error": fmt.Sprintf("%s token exchange failed: %s", label, string(text))})
			return
		}
		nested, _ := data["data"].(map[string]any)
		user, _ := nested["userInfo"].(map[string]any)
		for _, key := range []string{"accessToken", "refreshToken"} {
			value := firstNonEmpty(nested[key], data[key])
			if value != nil {
				tokens[key] = value
			}
		}
		tokens["email"] = ""
		if nodeTruthy(user["email"]) {
			tokens["email"] = user["email"]
		}
		if expiry := firstNonEmpty(nested["expiresAt"], data["expiresAt"]); expiry != nil {
			tokens["expires_at"] = expiry
		}
	}
	mapped := map[string]any{"provider": provider, "authType": "oauth", "testStatus": "active", "providerSpecificData": map[string]any{}, "expiresIn": float64(3600)}
	for _, key := range []string{"accessToken", "refreshToken", "email"} {
		if value, ok := tokens[key]; ok {
			mapped[key] = value
		}
	}
	specific := mapped["providerSpecificData"].(map[string]any)
	for _, key := range []string{"firstName", "lastName"} {
		if value, ok := tokens[key]; ok {
			specific[key] = value
		}
	}
	if nodeTruthy(tokens["expires_at"]) {
		if text, ok := tokens["expires_at"].(string); ok {
			if expiry, err := time.Parse(time.RFC3339Nano, text); err == nil {
				mapped["expiresIn"] = float64(int64(expiry.Sub(time.Now()).Seconds()))
			} else {
				nodeReply(w, 500, map[string]string{"error": "Invalid time value"})
				return
			}
		} else {
			nodeReply(w, 500, map[string]string{"error": "Invalid time value"})
			return
		}
	}
	mapped["expiresAt"] = nil
	if seconds := mapped["expiresIn"].(float64); seconds != 0 {
		mapped["expiresAt"] = time.Now().Add(time.Duration(seconds) * time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
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

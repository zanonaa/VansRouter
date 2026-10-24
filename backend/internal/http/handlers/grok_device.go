package handlers

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

var grokVersionPattern = regexp.MustCompile(`^\d+(?:\.\d+){1,3}$`)

func grokVersion() string {
	v := strings.TrimSpace(os.Getenv("GROK_CLI_VERSION"))
	if grokVersionPattern.MatchString(v) {
		return v
	}
	return "1.0.44"
}
func grokHeaders() map[string]string {
	v := grokVersion()
	return map[string]string{"Accept": "application/json", "Content-Type": "application/x-www-form-urlencoded", "User-Agent": "grok-pager/" + v + " grok-shell/" + v + " (linux; x86_64)"}
}
func (h *OAuthEngine) grokDevice(w http.ResponseWriter, r *http.Request) {
	c := h.config("grok-cli")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: grok-cli"})
		return
	}
	form := url.Values{"client_id": {c.ClientID}, "scope": {c.Scope}}
	if c.Referrer != "" {
		form.Set("referrer", c.Referrer)
	}
	status, data, raw, err := h.deviceJSON(r.Context(), "POST", c.DeviceCodeURL, grokHeaders(), strings.NewReader(form.Encode()))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Grok CLI device code request failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 500, map[string]string{"error": "Grok CLI device code request failed: " + string(raw)})
		return
	}
	if data == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid device code response"})
		return
	}
	verifier, _, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	data["codeVerifier"] = verifier
	nodeReply(w, 200, data)
}
func (h *OAuthEngine) grokPoll(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid or empty request body"})
		return
	}
	code, _ := body["deviceCode"].(string)
	verifier, _ := body["codeVerifier"].(string)
	if code == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing device code"})
		return
	}
	if verifier == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing code verifier"})
		return
	}
	c := h.config("grok-cli")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: grok-cli"})
		return
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "device_code": {code}, "client_id": {c.ClientID}}
	status, data, raw, err := h.deviceJSON(r.Context(), "POST", c.TokenURL, grokHeaders(), strings.NewReader(form.Encode()))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Token poll failed"})
		return
	}
	if data == nil {
		data = map[string]any{"error": "invalid_response", "error_description": string(raw)}
	}
	errorCode, _ := data["error"].(string)
	ok := (status >= 200 && status < 300) || errorCode == "authorization_pending" || errorCode == "slow_down"
	access, _ := data["access_token"].(string)
	if !ok || access == "" {
		result := map[string]any{"success": false}
		for source, target := range map[string]string{"error": "error", "error_description": "errorDescription"} {
			if value, exists := data[source]; exists {
				result[target] = value
			}
		}
		if ok {
			if errorCode == "authorization_pending" || errorCode == "slow_down" {
				result["pending"] = errorCode == "authorization_pending"
				if !nodeTruthy(result["errorDescription"]) && nodeTruthy(data["message"]) {
					result["errorDescription"] = data["message"]
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
	user := h.grokProfile(r.Context(), access)
	email := ""
	for _, claims := range []map[string]any{decodeJWTClaims(stringField(data, "id_token")), decodeJWTClaims(access)} {
		if email == "" {
			email = firstNonEmptyString(claims["email"])
		}
	}
	if email == "" {
		email = firstNonEmptyString(user["email"])
	}
	specific := map[string]any{"authMethod": "device_code", "idToken": nil, "email": nil, "userId": nil, "hasGrokCodeAccess": nil, "subscriptionTier": nil}
	if nodeTruthy(data["id_token"]) {
		specific["idToken"] = data["id_token"]
	}
	if email != "" {
		specific["email"] = email
	}
	specific["userId"] = firstNonEmpty(user["userId"], user["principalId"])
	for _, key := range []string{"hasGrokCodeAccess", "subscriptionTier"} {
		if value, ok := user[key]; ok {
			specific[key] = value
		}
	}
	mapped := map[string]any{"provider": "grok-cli", "authType": "oauth", "accessToken": access, "refreshToken": nil, "expiresAt": nil, "testStatus": "active", "providerSpecificData": specific}
	if nodeTruthy(data["refresh_token"]) {
		mapped["refreshToken"] = data["refresh_token"]
	}
	if value, ok := data["scope"]; ok {
		mapped["scope"] = value
	}
	if email != "" {
		mapped["email"] = email
	}
	display := strings.TrimSpace(strings.Join(nonEmptyStrings(user["firstName"], user["lastName"]), " "))
	if display != "" {
		mapped["displayName"] = display
	}
	if seconds, ok := data["expires_in"].(float64); ok {
		mapped["expiresIn"] = seconds
		if seconds != 0 {
			mapped["expiresAt"] = time.Now().Add(time.Duration(seconds * float64(time.Second))).UTC().Format("2006-01-02T15:04:05.000Z")
		}
	}
	saved, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"]}})
}
func nonEmptyStrings(values ...any) []string {
	out := []string{}
	for _, v := range values {
		if nodeTruthy(v) {
			out = append(out, jsString(v))
		}
	}
	return out
}
func (h *OAuthEngine) grokProfile(ctx context.Context, access string) map[string]any {
	target := "https://cli-chat-proxy.grok.com/v1/user"
	if c := h.config("grok-cli"); c != nil && c.UserInfoURL != "" {
		target = c.UserInfoURL
	}
	headers := grokHeaders()
	delete(headers, "Content-Type")
	headers["Authorization"] = "Bearer " + access
	headers["x-xai-token-auth"] = "xai-grok-cli"
	headers["x-grok-client-version"] = grokVersion()
	status, data, _, err := h.deviceJSON(ctx, "GET", target, headers, nil)
	if err != nil || status < 200 || status >= 300 || data == nil {
		return map[string]any{}
	}
	return data
}

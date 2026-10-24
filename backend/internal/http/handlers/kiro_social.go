package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

const kiroSocialBase = "https://prod.us-east-1.auth.desktop.kiro.dev/oauth/device/"

func kiroSocialClientID() string {
	if value := os.Getenv("KIRO_OAUTH_CLIENT_ID"); value != "" {
		return value
	}
	return "kiro-cli"
}
func (h *OAuthEngine) KiroSocialAuthorize(w http.ResponseWriter, r *http.Request) {
	provider := r.URL.Query().Get("provider")
	if provider != "google" && provider != "github" {
		nodeReply(w, 400, map[string]any{"error": "Invalid provider. Use 'google' or 'github'"})
		return
	}
	login := "Google"
	if provider == "github" {
		login = "Github"
	}
	encoded, _ := json.Marshal(map[string]any{"clientId": kiroSocialClientID(), "loginProvider": login})
	status, data, raw, err := h.deviceJSON(r.Context(), "POST", kiroSocialBase+"authorization", map[string]string{"Content-Type": "application/json"}, strings.NewReader(string(encoded)))
	if err != nil || data == nil && status >= 200 && status < 300 {
		nodeReply(w, 500, map[string]any{"error": "Internal server error"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 502, map[string]any{"error": "Device authorization failed: " + string(raw)})
		return
	}
	expires := pxpipeNum(data["expiresInMilliseconds"])
	if expires == 0 {
		expires = 300000
	}
	interval := pxpipeNum(data["intervalInMilliseconds"])
	if interval == 0 {
		interval = 5000
	}
	nodeReply(w, 200, map[string]any{"authUrl": firstNonEmpty(data["verificationUriComplete"], data["verificationUri"]), "deviceCode": data["deviceCode"], "userCode": data["userCode"], "expiresIn": math.Floor(expires / 1000), "interval": math.Floor(interval / 1000), "provider": provider})
}

func (h *OAuthEngine) KiroSocialExchange(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		nodeReply(w, 400, map[string]any{"error": "Invalid JSON body"})
		return
	}
	provider := ttsString(body["provider"])
	if !nodeTruthy(body["deviceCode"]) || provider != "google" && provider != "github" {
		nodeReply(w, 400, map[string]any{"error": "Missing or invalid deviceCode or provider"})
		return
	}
	encoded, _ := json.Marshal(map[string]any{"deviceCode": body["deviceCode"], "clientId": kiroSocialClientID()})
	status, data, _, err := h.deviceJSON(r.Context(), "POST", kiroSocialBase+"poll", map[string]string{"Content-Type": "application/json"}, strings.NewReader(string(encoded)))
	fail := func() { nodeReply(w, 500, map[string]any{"error": "Internal server error"}) }
	if err != nil {
		fail()
		return
	}
	if data == nil {
		if status == 0 {
			status = 500
		}
		nodeReply(w, status, map[string]any{"success": false, "pending": false, "error": "invalid_response"})
		return
	}
	progress := data["error"]
	if progress == nil {
		progress = data["status"]
	}
	if progress == "authorization_pending" || progress == "slow_down" {
		nodeReply(w, 200, map[string]any{"success": false, "pending": true, "error": progress})
		return
	}
	if status < 200 || status >= 300 || nodeTruthy(data["error"]) {
		code := strings.TrimSpace(ttsString(data["error"]))
		if code == "" {
			code = "authorization_failed"
		}
		if status < 400 || status > 599 {
			status = 400
		}
		nodeReply(w, status, map[string]any{"success": false, "pending": false, "error": code})
		return
	}
	if !nodeTruthy(data["accessToken"]) && !nodeTruthy(data["refreshToken"]) {
		nodeReply(w, 502, map[string]any{"success": false, "pending": false, "error": "invalid_token_response"})
		return
	}
	claims := decodeJWTClaims(ttsString(data["accessToken"]))
	email := firstNonEmpty(claims["email"], claims["preferred_username"], claims["sub"])
	providerData := map[string]any{"authMethod": "imported", "provider": strings.ToUpper(provider[:1]) + provider[1:]}
	if nodeTruthy(data["profileArn"]) {
		providerData["profileArn"] = data["profileArn"]
	}
	target := ttsString(body["targetProvider"])
	if target != "kiro" && !strings.HasPrefix(target, "kiro-") {
		target = "kiro"
	}
	expires := pxpipeNum(data["expiresIn"])
	if expires == 0 {
		expires = 3600
	}
	record := map[string]any{"accessToken": data["accessToken"], "refreshToken": data["refreshToken"], "expiresAt": time.Now().Add(time.Duration(expires * float64(time.Second))).UTC().Format("2006-01-02T15:04:05.000Z"), "email": email, "providerSpecificData": providerData, "testStatus": "active", "isActive": true}
	if h.ListConnections == nil || h.UpdateConnection == nil {
		fail()
		return
	}
	existing, err := h.ListConnections(r.Context(), target, nil)
	if err != nil {
		fail()
		return
	}
	match := findKiroSocialIdentity(existing, ttsString(data["profileArn"]), ttsString(email))
	var saved map[string]any
	if match != nil {
		saved, err = h.UpdateConnection(r.Context(), ttsString(match["id"]), record)
	} else {
		record["provider"] = target
		record["authType"] = "oauth"
		saved, err = h.CreateConnection(r.Context(), record)
	}
	if err != nil {
		fail()
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"], "email": saved["email"]}})
}

func findKiroSocialIdentity(existing []map[string]any, profile, email string) map[string]any {
	candidates := []map[string]any{}
	for _, connection := range existing {
		if strings.EqualFold(strings.TrimSpace(ttsString(connection["authType"])), "oauth") {
			candidates = append(candidates, connection)
		}
	}
	email = strings.TrimSpace(email)
	if profile != "" {
		for _, connection := range candidates {
			metadata, _ := connection["providerSpecificData"].(map[string]any)
			if strings.TrimSpace(ttsString(metadata["profileArn"])) == profile {
				oldEmail := strings.TrimSpace(ttsString(connection["email"]))
				if email != "" && (oldEmail == "" || strings.EqualFold(oldEmail, email)) {
					return connection
				}
				break
			}
		}
	}
	if email != "" {
		for _, connection := range candidates {
			if strings.EqualFold(strings.TrimSpace(ttsString(connection["email"])), email) {
				return connection
			}
		}
	}
	return nil
}

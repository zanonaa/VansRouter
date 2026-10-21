package handlers

import (
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (h *OAuthEngine) qoderDevice(w http.ResponseWriter, r *http.Request) {
	c := h.config("qoder")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: qoder"})
		return
	}
	verifier, challenge, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	nonce, machine := uuid.NewString(), uuid.NewString()
	query := url.Values{"challenge": {challenge}, "challenge_method": {"S256"}, "machine_id": {machine}, "nonce": {nonce}}
	nodeReply(w, 200, map[string]any{"device_code": nonce, "user_code": strings.ToUpper(nonce[:8]), "verification_uri": c.LoginURL, "verification_uri_complete": c.LoginURL + "?" + query.Encode(), "expires_in": 300, "interval": 2, "codeVerifier": verifier, "_qoderNonce": nonce, "_qoderMachineId": machine})
}
func qoderExpiry(value, seconds any, now time.Time) time.Time {
	if n, ok := value.(float64); ok && !math.IsNaN(n) && !math.IsInf(n, 0) && n > 0 {
		return time.UnixMilli(int64(n))
	}
	if text, ok := value.(string); ok {
		trim := strings.TrimSpace(text)
		if trim != "" {
			digits := true
			for _, r := range trim {
				if r < '0' || r > '9' {
					digits = false
					break
				}
			}
			if digits {
				if n, err := strconv.ParseInt(trim, 10, 64); err == nil && n > 0 {
					return time.UnixMilli(n)
				}
			}
			if parsed, err := time.Parse(time.RFC3339Nano, trim); err == nil {
				return parsed
			}
		}
	}
	if n, ok := seconds.(float64); ok && n >= 0 && !math.IsInf(n, 0) {
		return now.Add(time.Duration(n * float64(time.Second)))
	}
	return now.Add(30 * 24 * time.Hour)
}
func (h *OAuthEngine) qoderPoll(w http.ResponseWriter, r *http.Request) {
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
	c := h.config("qoder")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: qoder"})
		return
	}
	failure := func(message string) {
		nodeReply(w, 200, map[string]any{"success": false, "error": "poll_failed", "errorDescription": message})
	}
	query := url.Values{"nonce": {code}, "verifier": {verifier}, "challenge_method": {"S256"}}
	status, data, _, err := h.deviceJSON(r.Context(), "GET", c.DeviceTokenURL+"?"+query.Encode(), map[string]string{"Accept": "application/json", "User-Agent": "Go-http-client/2.0"}, nil)
	if err != nil {
		failure("Qoder device token poll failed")
		return
	}
	if status == 202 || status == 404 {
		nodeReply(w, 200, map[string]any{"success": false, "error": "authorization_pending"})
		return
	}
	if status < 200 || status >= 300 {
		message := "HTTP " + strconv.Itoa(status)
		if nodeTruthy(data["message"]) {
			message = jsString(data["message"])
		}
		failure("Qoder device token poll failed: " + message)
		return
	}
	if data == nil {
		failure("Qoder device token poll: invalid JSON response")
		return
	}
	if !nodeTruthy(data["token"]) {
		failure("Qoder device token poll returned 200 but no token")
		return
	}
	access := jsString(data["token"])
	profile := map[string]any{}
	userStatus, user, _, err := h.deviceJSON(r.Context(), "GET", c.UserInfoURL, map[string]string{"Authorization": "Bearer " + access, "Accept": "application/json", "User-Agent": "Go-http-client/2.0"}, nil)
	if err == nil && userStatus >= 200 && userStatus < 300 && user != nil {
		profile = user
	}
	now := time.Now()
	expiry := qoderExpiry(data["expires_at"], data["expires_in"], now)
	seconds := math.Max(86400, math.Floor(expiry.Sub(now).Seconds()))
	extra, _ := body["extraData"].(map[string]any)
	machine := ""
	if nodeTruthy(extra["_qoderMachineId"]) {
		machine = jsString(extra["_qoderMachineId"])
	}
	userID := ""
	if nodeTruthy(data["user_id"]) {
		userID = jsString(data["user_id"])
	}
	email := firstNonEmptyString(profile["email"])
	var emailValue any
	if email != "" {
		emailValue = email
	} else if userID != "" {
		emailValue = "qoder-user-" + userID
	}
	var display any
	if name := firstNonEmptyString(profile["name"], profile["username"]); name != "" {
		display = name
	}
	var refresh any
	if nodeTruthy(data["refresh_token"]) {
		refresh = data["refresh_token"]
	}
	mapped := map[string]any{"provider": "qoder", "authType": "oauth", "accessToken": access, "refreshToken": refresh, "expiresIn": seconds, "expiresAt": now.Add(time.Duration(seconds * float64(time.Second))).UTC().Format("2006-01-02T15:04:05.000Z"), "email": emailValue, "displayName": display, "testStatus": "active", "providerSpecificData": map[string]any{"authMethod": "device", "userId": userID, "machineId": machine, "organizationId": firstNonEmptyString(profile["organization_id"])}}
	saved, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"]}})
}

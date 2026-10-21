package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func codebuddyHeaders(domain, userAgent string, poll bool) map[string]string {
	h := map[string]string{
		"Accept": "application/json", "User-Agent": userAgent, "X-Requested-With": "XMLHttpRequest",
		"X-Domain": domain, "X-No-Authorization": "true", "X-No-User-Id": "true", "X-Product": "SaaS",
	}
	if !poll {
		h["Content-Type"] = "application/json"
	}
	if poll {
		h["X-No-Enterprise-Id"] = "true"
		h["X-No-Department-Info"] = "true"
	}
	return h
}

var codebuddyDomains = map[string]string{"codebuddy-cn": "copilot.tencent.com", "codebuddy-intl": "www.codebuddy.ai"}

func (h *OAuthEngine) codebuddyDevice(w http.ResponseWriter, r *http.Request, provider string) {
	c := h.config(provider)
	if c == nil || c.StateURL == "" {
		nodeReply(w, 400, map[string]string{"error": "Provider " + provider + " does not support device code flow"})
		return
	}
	status, data, raw, err := h.deviceJSON(r.Context(), "POST", c.StateURL+"?platform="+url.QueryEscape(c.Platform), codebuddyHeaders(codebuddyDomains[provider], c.UserAgent, false), strings.NewReader("{}"))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "CodeBuddy state request failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 500, map[string]string{"error": fmt.Sprintf("CodeBuddy state request failed: %s", string(raw))})
		return
	}
	if code, ok := data["code"].(float64); !ok || code != 0 {
		msg := "missing state/authUrl"
		if nodeTruthy(data["msg"]) {
			msg = jsString(data["msg"])
		}
		nodeReply(w, 500, map[string]string{"error": "CodeBuddy state error: " + msg})
		return
	}
	inner, _ := data["data"].(map[string]any)
	if inner == nil || !nodeTruthy(inner["state"]) || !nodeTruthy(inner["authUrl"]) {
		nodeReply(w, 500, map[string]string{"error": "CodeBuddy state error: missing state/authUrl"})
		return
	}
	verifier, _, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to generate PKCE"})
		return
	}
	nodeReply(w, 200, map[string]any{"device_code": inner["state"], "verification_uri": inner["authUrl"], "user_code": "", "interval": c.PollInterval / 1000, "_isCodeBuddy": true, "codeVerifier": verifier})
}

func (h *OAuthEngine) codebuddyPoll(w http.ResponseWriter, r *http.Request, provider string) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid or empty request body"})
		return
	}
	deviceCode, _ := body["deviceCode"].(string)
	if deviceCode == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing device code"})
		return
	}
	c := h.config(provider)
	if c == nil || c.TokenURL == "" {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: " + provider})
		return
	}
	status, data, _, err := h.deviceJSON(r.Context(), "GET", c.TokenURL+"?state="+url.QueryEscape(deviceCode), codebuddyHeaders(codebuddyDomains[provider], c.UserAgent, true), nil)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Poll failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 200, map[string]any{"success": false, "error": "request_failed"})
		return
	}
	code, validCode := data["code"].(float64)
	inner, _ := data["data"].(map[string]any)
	if validCode && code == 0 && inner != nil && nodeTruthy(inner["accessToken"]) {
		expiresIn := 86400.0
		if v, ok := inner["expiresIn"].(float64); ok && v != 0 {
			expiresIn = v
		}
		refresh := ""
		if nodeTruthy(inner["refreshToken"]) {
			refresh = jsString(inner["refreshToken"])
		}
		mapped := map[string]any{
			"provider": provider, "authType": "oauth", "testStatus": "active",
			"accessToken": inner["accessToken"], "refreshToken": refresh,
			"expiresIn": expiresIn, "providerSpecificData": map[string]any{},
		}
		mapped["expiresAt"] = time.Now().Add(time.Duration(expiresIn * float64(time.Second))).UTC().Format("2006-01-02T15:04:05.000Z")
		saved, err := h.CreateConnection(r.Context(), mapped)
		if err != nil {
			nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
			return
		}
		nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"]}})
		return
	}
	if code == 11217 {
		nodeReply(w, 200, map[string]any{"success": false, "pending": true, "error": "authorization_pending"})
		return
	}
	msg := "unknown_error"
	if nodeTruthy(data["msg"]) {
		msg = jsString(data["msg"])
	}
	nodeReply(w, 200, map[string]any{"success": false, "error": msg})
}

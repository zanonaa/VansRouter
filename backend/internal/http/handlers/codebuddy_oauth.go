package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func codebuddyHeaders(domain string, poll bool) map[string]string {
	h := map[string]string{
		"Accept": "application/json", "User-Agent": "CodeBuddy", "X-Requested-With": "XMLHttpRequest",
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
	if c == nil || c.DeviceCodeURL == "" {
		nodeReply(w, 400, map[string]string{"error": "Provider " + provider + " does not support device code flow"})
		return
	}
	platform := c.DeviceScope
	if platform == "" {
		platform = "default"
	}
	status, data, raw, err := h.deviceJSON(r.Context(), "POST", c.DeviceCodeURL+"?platform="+url.QueryEscape(platform), codebuddyHeaders(codebuddyDomains[provider], false), strings.NewReader("{}"))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "CodeBuddy state request failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 500, map[string]string{"error": fmt.Sprintf("CodeBuddy state request failed: %s", string(raw))})
		return
	}
	if code, _ := data["code"].(float64); code != 0 {
		msg := jsString(data["msg"])
		if msg == "" {
			msg = "missing state/authUrl"
		}
		nodeReply(w, 500, map[string]string{"error": "CodeBuddy state error: " + msg})
		return
	}
	inner, _ := data["data"].(map[string]any)
	if inner == nil || !nodeTruthy(inner["state"]) || !nodeTruthy(inner["authUrl"]) {
		nodeReply(w, 500, map[string]string{"error": "CodeBuddy state error: missing state/authUrl"})
		return
	}
	interval := 3.0
	if c.UserAgent != "" {
		// pollInterval stored in UserAgent field by exporter convention for this provider
	}
	nodeReply(w, 200, map[string]any{"device_code": inner["state"], "verification_uri": inner["authUrl"], "user_code": "", "interval": interval})
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
	status, data, _, err := h.deviceJSON(r.Context(), "GET", c.TokenURL+"?state="+url.QueryEscape(deviceCode), codebuddyHeaders(codebuddyDomains[provider], true), nil)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Poll failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 200, map[string]any{"success": false, "error": "request_failed"})
		return
	}
	code, _ := data["code"].(float64)
	inner, _ := data["data"].(map[string]any)
	if code == 0 && inner != nil && nodeTruthy(inner["accessToken"]) {
		expiresIn := 86400.0
		if v, ok := inner["expiresIn"].(float64); ok && v != 0 {
			expiresIn = v
		}
		mapped := map[string]any{
			"provider": provider, "authType": "oauth", "testStatus": "active",
			"accessToken": inner["accessToken"], "refreshToken": jsString(inner["refreshToken"]),
			"expiresIn": expiresIn, "providerSpecificData": map[string]any{},
		}
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
	msg := jsString(data["msg"])
	if msg == "" {
		msg = "unknown_error"
	}
	nodeReply(w, 200, map[string]any{"success": false, "error": msg})
	_ = json.Marshal
}

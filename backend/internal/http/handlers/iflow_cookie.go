package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf16"
)

var bxAuthCookie = regexp.MustCompile(`BXAuth=([^;]+)`)

func (h *OAuthEngine) IflowCookie(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid request body"})
		return
	}
	cookie, ok := body["cookie"].(string)
	if !ok || cookie == "" {
		nodeReply(w, 400, map[string]string{"error": "Cookie is required"})
		return
	}
	cookie = strings.TrimSpace(cookie)
	if !strings.Contains(cookie, "BXAuth=") {
		nodeReply(w, 400, map[string]string{"error": "Cookie must contain BXAuth field"})
		return
	}
	if !strings.HasSuffix(cookie, ";") {
		cookie += ";"
	}
	target := "https://platform.iflow.cn/api/openapi/apikey"
	if c := h.config("iflow"); c != nil && c.CookieKeyURL != "" {
		target = c.CookieKeyURL
	}
	headers := map[string]string{"Cookie": cookie, "Accept": "application/json, text/plain, */*", "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36", "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8", "Connection": "keep-alive", "Sec-Fetch-Dest": "empty", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Site": "same-origin"}
	// Não anuncia br sem decoder: net/http negocia gzip e descomprime sozinho.
	status, data, raw, err := h.deviceJSON(r.Context(), "GET", target, headers, nil)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch API key info"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, status, map[string]string{"error": "Failed to fetch API key info: " + string(raw)})
		return
	}
	if data == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid API key info response"})
		return
	}
	if !nodeTruthy(data["success"]) {
		message := "undefined"
		if v, ok := data["message"]; ok {
			message = jsString(v)
		}
		nodeReply(w, 400, map[string]string{"error": "API key fetch failed: " + message})
		return
	}
	keyData, _ := data["data"].(map[string]any)
	if !nodeTruthy(keyData["name"]) {
		nodeReply(w, 400, map[string]string{"error": "Missing name in API key info"})
		return
	}
	delete(headers, "Sec-Fetch-Dest")
	delete(headers, "Sec-Fetch-Mode")
	delete(headers, "Sec-Fetch-Site")
	headers["Content-Type"] = "application/json"
	headers["Origin"] = "https://platform.iflow.cn"
	headers["Referer"] = "https://platform.iflow.cn/"
	payload, _ := json.Marshal(map[string]any{"name": keyData["name"]})
	status, data, raw, err = h.deviceJSON(r.Context(), "POST", target, headers, strings.NewReader(string(payload)))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to refresh API key"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, status, map[string]string{"error": "Failed to refresh API key: " + string(raw)})
		return
	}
	if data == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid API key refresh response"})
		return
	}
	if !nodeTruthy(data["success"]) {
		message := "undefined"
		if v, ok := data["message"]; ok {
			message = jsString(v)
		}
		nodeReply(w, 400, map[string]string{"error": "API key refresh failed: " + message})
		return
	}
	refreshed, _ := data["data"].(map[string]any)
	if !nodeTruthy(refreshed["apiKey"]) {
		nodeReply(w, 400, map[string]string{"error": "Missing API key in response"})
		return
	}
	key, ok := refreshed["apiKey"].(string)
	if !ok {
		nodeReply(w, 500, map[string]string{"error": "Invalid API key response"})
		return
	}
	storedCookie := ""
	if match := bxAuthCookie.FindStringSubmatch(cookie); len(match) > 1 {
		storedCookie = "BXAuth=" + match[1] + ";"
	}
	specific := map[string]any{"cookie": storedCookie}
	if expiry, exists := refreshed["expireTime"]; exists {
		specific["expireTime"] = expiry
	}
	name := firstNonEmpty(refreshed["name"], keyData["name"])
	saved, err := h.CreateConnection(r.Context(), map[string]any{"provider": "iflow", "authType": "cookie", "name": name, "email": name, "apiKey": key, "providerSpecificData": specific, "testStatus": "active", "isActive": true})
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	units := utf16.Encode([]rune(key))
	if len(units) > 10 {
		units = units[:10]
	}
	result := map[string]any{"id": saved["id"], "provider": saved["provider"], "email": saved["email"], "apiKey": string(utf16.Decode(units)) + "..."}
	if expiry, exists := refreshed["expireTime"]; exists {
		result["expireTime"] = expiry
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": result})
}

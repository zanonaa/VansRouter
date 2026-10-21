package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var freebuffAuthCode = regexp.MustCompile(`auth_code=([^&]+)`)

func (h *OAuthEngine) freebuffDevice(w http.ResponseWriter, r *http.Request) {
	c := h.config("freebuff")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: freebuff"})
		return
	}
	fingerprint := uuid.NewString()
	base := c.BaseURL
	if base == "" {
		base = "https://freebuff.com"
	}
	path := c.LoginCodePath
	if path == "" {
		path = "/api/auth/cli/code"
	}
	raw, _ := json.Marshal(map[string]string{"fingerprintId": fingerprint})
	status, data, text, err := h.deviceJSON(r.Context(), "POST", strings.TrimSuffix(base, "/")+path, map[string]string{"Content-Type": "application/json", "Accept": "application/json", "User-Agent": "codebuff-cli/0.0.138"}, strings.NewReader(string(raw)))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Freebuff login code request failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 500, map[string]string{"error": "Freebuff login code request failed: " + string(text)})
		return
	}
	if data == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid Freebuff login response"})
		return
	}
	login, _ := data["loginUrl"].(string)
	authCode := ""
	if match := freebuffAuthCode.FindStringSubmatch(login); len(match) > 1 {
		authCode = match[1]
	}
	expiresAt := float64(0)
	switch value := data["expiresAt"].(type) {
	case float64:
		expiresAt = value
	case string:
		expiresAt, _ = strconv.ParseFloat(value, 64)
	}
	timeout := c.OAuthTimeoutMS
	if timeout == 0 {
		timeout = 300000
	}
	seconds := math.Max(60, math.Floor(timeout/1000))
	remaining := seconds
	if expiresAt > 0 {
		remaining = math.Floor((expiresAt - float64(time.Now().UnixMilli())) / 1000)
	}
	remaining = math.Max(60, math.Min(remaining, seconds))
	identity := map[string]any{"fingerprintId": fingerprint, "expiresAt": expiresAt}
	if nodeTruthy(data["fingerprintId"]) {
		identity["fingerprintId"] = data["fingerprintId"]
	}
	if value, ok := data["fingerprintHash"]; ok {
		identity["fingerprintHash"] = value
	}
	encoded, _ := json.Marshal(identity)
	verifier, _, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	nodeReply(w, 200, map[string]any{"device_code": string(encoded), "user_code": authCode, "verification_uri": login, "verification_uri_complete": login, "expires_in": remaining, "interval": 5, "codeVerifier": verifier})
}
func (h *OAuthEngine) freebuffPoll(w http.ResponseWriter, r *http.Request) {
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
	pending := func() {
		nodeReply(w, 200, map[string]any{"success": false, "error": "authorization_pending", "pending": true})
	}
	var identity map[string]any
	_ = json.Unmarshal([]byte(code), &identity)
	if !nodeTruthy(identity["fingerprintId"]) || !nodeTruthy(identity["fingerprintHash"]) || !nodeTruthy(identity["expiresAt"]) {
		pending()
		return
	}
	c := h.config("freebuff")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: freebuff"})
		return
	}
	base := c.BaseURL
	if base == "" {
		base = "https://freebuff.com"
	}
	path := c.LoginStatusPath
	if path == "" {
		path = "/api/auth/cli/status"
	}
	query := url.Values{}
	for _, key := range []string{"fingerprintId", "fingerprintHash", "expiresAt"} {
		query.Set(key, jsString(identity[key]))
	}
	_, data, _, err := h.deviceJSON(r.Context(), "GET", strings.TrimSuffix(base, "/")+path+"?"+query.Encode(), map[string]string{"Accept": "application/json", "User-Agent": "codebuff-cli/0.0.138"}, nil)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Freebuff status request failed"})
		return
	}
	user, _ := data["user"].(map[string]any)
	if !nodeTruthy(user["authToken"]) {
		pending()
		return
	}
	mapped := map[string]any{"provider": "freebuff", "authType": "oauth", "testStatus": "active", "accessToken": user["authToken"], "refreshToken": nil, "expiresAt": nil}
	for source, target := range map[string]string{"email": "email", "name": "displayName"} {
		if nodeTruthy(user[source]) {
			mapped[target] = user[source]
		}
	}
	specific := map[string]any{"authMethod": "device_code", "fingerprintId": nil, "userId": nil}
	if nodeTruthy(user["fingerprintId"]) {
		specific["fingerprintId"] = user["fingerprintId"]
	}
	if nodeTruthy(user["id"]) {
		specific["userId"] = user["id"]
	}
	mapped["providerSpecificData"] = specific
	saved, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"]}})
}

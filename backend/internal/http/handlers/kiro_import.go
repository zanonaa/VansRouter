package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// KiroImport valida o refresh token na origem e persiste a conexão OAuth.
func (h *OAuthEngine) KiroImport(w http.ResponseWriter, r *http.Request) {
	fail := func(err error) { nodeReply(w, 500, map[string]any{"error": err.Error()}) }
	body, err := poolInput(r)
	if err != nil {
		fail(err)
		return
	}
	refresh, ok := body["refreshToken"].(string)
	if !ok || refresh == "" {
		nodeReply(w, 400, map[string]any{"error": "Refresh token is required"})
		return
	}
	refresh = strings.TrimSpace(refresh)
	idc := nodeTruthy(body["clientId"]) && nodeTruthy(body["clientSecret"])
	endpoint := "https://prod.us-east-1.auth.desktop.kiro.dev/refreshToken"
	requestBody := map[string]any{"refreshToken": refresh}
	region := "us-east-1"
	if value, ok := body["region"].(string); ok && value != "" {
		region = value
	}
	if idc {
		if !awsRegion.MatchString(region) {
			fail(fmt.Errorf("Invalid region"))
			return
		}
		if unsupportedKiro.MatchString(region) {
			fail(fmt.Errorf("Unsupported Kiro region: %s", region))
			return
		}
		endpoint = "https://oidc." + region + ".amazonaws.com/token"
		requestBody["clientId"] = body["clientId"]
		requestBody["clientSecret"] = body["clientSecret"]
		requestBody["grantType"] = "refresh_token"
	}
	encoded, _ := json.Marshal(requestBody)
	status, tokens, raw, err := h.deviceJSON(r.Context(), "POST", endpoint, map[string]string{"Content-Type": "application/json"}, strings.NewReader(string(encoded)))
	if err != nil {
		fail(err)
		return
	}
	if status < 200 || status >= 300 {
		fail(fmt.Errorf("Token refresh failed: %s", raw))
		return
	}
	if tokens == nil {
		fail(fmt.Errorf("Invalid token response"))
		return
	}
	claims := decodeJWTClaims(ttsString(tokens["accessToken"]))
	email := firstNonEmpty(claims["email"], claims["preferred_username"], claims["sub"])
	data := map[string]any{"profileArn": firstNonEmpty(body["profileArn"], tokens["profileArn"]), "authMethod": "imported", "provider": "Imported"}
	if idc {
		data["authMethod"] = "idc"
		data["provider"] = "Enterprise"
		data["clientId"] = body["clientId"]
		data["clientSecret"] = body["clientSecret"]
		data["region"] = region
	}
	expires := pxpipeNum(tokens["expiresIn"])
	if expires == 0 {
		expires = 3600
	}
	record := map[string]any{"provider": "kiro", "authType": "oauth", "accessToken": tokens["accessToken"], "refreshToken": firstNonEmpty(tokens["refreshToken"], refresh), "expiresAt": time.Now().Add(time.Duration(expires * float64(time.Second))).UTC().Format("2006-01-02T15:04:05.000Z"), "email": email, "providerSpecificData": data, "testStatus": "active"}
	saved, err := h.CreateConnection(r.Context(), record)
	if err != nil {
		fail(err)
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"], "email": saved["email"]}})
}

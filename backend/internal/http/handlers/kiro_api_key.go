package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"time"
)

var awsRegion = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*-\d{1,2}$`)
var unsupportedKiro = regexp.MustCompile(`^(?:cn|us-gov|us-iso|us-isob)-`)

func validKiroRegion(value string) bool {
	return awsRegion.MatchString(value) && !unsupportedKiro.MatchString(value)
}
func (h *OAuthEngine) KiroAPIKey(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "API key validation failed"}) }
	body, err := poolInput(r)
	if err != nil || body == nil {
		fail()
		return
	}
	key, ok := body["apiKey"].(string)
	if !ok || strings.TrimSpace(key) == "" {
		nodeReply(w, 400, map[string]string{"error": "API key is required"})
		return
	}
	key = strings.TrimSpace(key)
	region := "us-east-1"
	if nodeTruthy(body["region"]) {
		value, ok := body["region"].(string)
		if !ok {
			fail()
			return
		}
		region = value
	}
	// Límite de confiança antes da interpolação no hostname AWS.
	if !validKiroRegion(region) {
		fail()
		return
	}
	endpoint := "https://codewhisperer." + region + ".amazonaws.com"
	if c := h.config("kiro"); c != nil && c.ProfileURL != "" {
		endpoint = c.ProfileURL
	}
	status, data, _, err := h.deviceJSON(r.Context(), "POST", endpoint, map[string]string{"Content-Type": "application/x-amz-json-1.0", "x-amz-target": "AmazonCodeWhispererService.ListAvailableProfiles", "Authorization": "Bearer " + key, "Accept": "application/json"}, strings.NewReader(`{"maxResults":10}`))
	if err != nil || status < 200 || status >= 300 || data == nil {
		fail()
		return
	}
	profiles, _ := data["profiles"].([]any)
	var selected any
	for index, raw := range profiles {
		profile, _ := raw.(map[string]any)
		arn := firstNonEmpty(profile["arn"], profile["profileArn"])
		if index == 0 {
			selected = arn
		}
		if text, ok := arn.(string); ok {
			parts := strings.Split(text, ":")
			if len(parts) > 3 && parts[3] == region {
				selected = arn
				break
			}
		}
	}
	claims := decodeJWTClaims(key)
	email := firstNonEmpty(claims["email"], claims["preferred_username"], claims["sub"])
	mapped := map[string]any{"provider": "kiro", "authType": "api_key", "accessToken": key, "refreshToken": nil, "expiresAt": time.Now().Add(365 * 24 * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z"), "email": email, "providerSpecificData": map[string]any{"profileArn": selected, "region": region, "authMethod": "api_key", "provider": "API Key"}, "testStatus": "active"}
	saved, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		fail()
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"], "email": saved["email"]}})
}

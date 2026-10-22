package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func normalizeKiroExternalAuth(raw any) (map[string]any, error) {
	if text, ok := raw.(string); ok {
		if json.Unmarshal([]byte(text), &raw) != nil {
			return nil, fmt.Errorf("CLIProxyAPI auth JSON is invalid")
		}
	}
	input, ok := raw.(map[string]any)
	if !ok || input == nil {
		return nil, fmt.Errorf("CLIProxyAPI auth JSON is required")
	}
	text := func(keys ...string) string { return strings.TrimSpace(ttsString(firstNonEmptyValues(input, keys...))) }
	method := text("auth_method", "authMethod")
	if method != "" && method != "external_idp" {
		return nil, fmt.Errorf("Only external_idp Kiro auth is supported by this importer")
	}
	endpoint := text("token_endpoint", "tokenEndpoint")
	if endpoint == "" {
		return nil, fmt.Errorf("token_endpoint is required")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("token_endpoint must be a valid URL")
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("token_endpoint must use https")
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "login.microsoftonline.com", "login.microsoft.com", "login.windows.net":
	default:
		return nil, fmt.Errorf("token_endpoint must be a Microsoft login endpoint")
	}
	region := text("region")
	if region == "" {
		region = "us-east-1"
	}
	if !awsRegion.MatchString(region) {
		return nil, fmt.Errorf("Invalid region")
	}
	if unsupportedKiro.MatchString(region) {
		return nil, fmt.Errorf("Unsupported Kiro region: %s", region)
	}
	scope := text("scopes", "scope")
	if values, ok := firstNonEmptyValues(input, "scopes", "scope").([]any); ok {
		list := []string{}
		for _, value := range values {
			if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
				list = append(list, strings.TrimSpace(s))
			}
		}
		scope = strings.Join(list, " ")
	}
	access := text("access_token", "accessToken")
	refresh := text("refresh_token", "refreshToken")
	client := text("client_id", "clientId")
	profile := text("profile_arn", "profileArn")
	for _, required := range []struct{ name, value string }{{"access_token", access}, {"refresh_token", refresh}, {"client_id", client}, {"scopes", scope}, {"profile_arn", profile}} {
		if required.value == "" {
			return nil, fmt.Errorf("%s is required", required.name)
		}
	}
	claims := decodeJWTClaims(access)
	email := firstNonEmpty(input["email"], claims["email"], claims["preferred_username"], claims["upn"], claims["sub"])
	expiresAt := time.Now().Add(time.Hour)
	if exp := pxpipeNum(claims["exp"]); exp != 0 {
		expiresAt = time.UnixMilli(int64(exp * 1000))
	}
	if seconds := floatAnyOrZero(firstNonEmptyValues(input, "expires_in", "expiresIn")); seconds > 0 {
		expiresAt = time.Now().Add(time.Duration(seconds * float64(time.Second)))
	}
	if explicit := text("expired", "expires_at", "expiresAt"); explicit != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, explicit); err == nil {
			expiresAt = parsed
		}
	}
	return map[string]any{"provider": "kiro", "authType": "oauth", "accessToken": access, "refreshToken": refresh, "email": email, "expiresAt": expiresAt.UTC().Format("2006-01-02T15:04:05.000Z"), "testStatus": "active", "providerSpecificData": map[string]any{"profileArn": profile, "region": region, "authMethod": "external_idp", "provider": "CLIProxyAPI", "clientId": client, "tokenEndpoint": parsed.String(), "scope": scope}}, nil
}

func firstNonEmptyValues(input map[string]any, keys ...string) any {
	values := make([]any, 0, len(keys))
	for _, key := range keys {
		values = append(values, input[key])
	}
	return firstNonEmpty(values...)
}

func (h *OAuthEngine) KiroCLIImport(w http.ResponseWriter, r *http.Request) {
	fail := func(err error) { nodeReply(w, 400, map[string]any{"error": err.Error()}) }
	body, err := poolInput(r)
	if err != nil {
		fail(err)
		return
	}
	var raw any = body
	for _, key := range []string{"cliProxyAuth", "auth", "json"} {
		if value, exists := body[key]; exists && value != nil {
			raw = value
			break
		}
	}
	record, err := normalizeKiroExternalAuth(raw)
	if err != nil {
		fail(err)
		return
	}
	saved, err := h.CreateConnection(r.Context(), record)
	if err != nil {
		fail(err)
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"], "email": saved["email"]}})
}

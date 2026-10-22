package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// refreshConnection implements the refresh protocols explicitly represented
// here. Unsupported provider protocols are not guessed from an OAuth URL.
func (t *TranslatorSend) refreshConnection(ctx context.Context, provider string, connection map[string]any) (map[string]any, error) {
	refresh := ttsString(connection["refreshToken"])
	if refresh == "" {
		return nil, nil
	}
	switch provider {
	case "claude", "codex", "gemini-cli", "antigravity", "qwen":
	default:
		return nil, nil
	}
	config := providers.GetOAuthConfig(provider)
	if config == nil || config.TokenURL == "" {
		return nil, nil
	}
	payload := map[string]string{"grant_type": "refresh_token", "refresh_token": refresh, "client_id": config.ClientID}
	contentType := "application/x-www-form-urlencoded"
	var encoded []byte
	if provider == "claude" || provider == "codex" {
		contentType = "application/json"
		encoded, _ = json.Marshal(payload)
	} else {
		form := url.Values{}
		for k, v := range payload {
			form.Set(k, v)
		}
		if provider == "gemini-cli" || provider == "antigravity" {
			form.Set("client_secret", config.ClientSecret)
		}
		encoded = []byte(form.Encode())
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	client, relay, cleanup, err := connectionHTTP(ctx, metadata, t.GetPool, t.Client)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	if err := rewriteConnectionRelay(request, relay); err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var tokens map[string]any
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
		return nil, nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&tokens); err != nil {
		return nil, err
	}
	access := ttsString(tokens["access_token"])
	if strings.TrimSpace(access) == "" {
		return nil, nil
	}
	patch := map[string]any{"accessToken": access, "refreshToken": refresh, "lastRefreshAt": time.Now().UTC().Format("2006-01-02T15:04:05.000Z")}
	for source, target := range map[string]string{"refresh_token": "refreshToken", "id_token": "idToken"} {
		if value := ttsString(tokens[source]); value != "" {
			patch[target] = value
		}
	}
	if seconds := pxpipeNum(tokens["expires_in"]); seconds > 0 {
		patch["expiresIn"] = seconds
		patch["expiresAt"] = time.Now().Add(time.Duration(seconds * float64(time.Second))).UTC().Format("2006-01-02T15:04:05.000Z")
	}
	if resource := ttsString(tokens["resource_url"]); resource != "" {
		metadata := map[string]any{}
		existing, _ := connection["providerSpecificData"].(map[string]any)
		for k, v := range existing {
			metadata[k] = v
		}
		metadata["resourceUrl"] = resource
		patch["providerSpecificData"] = metadata
	}
	if _, err := t.Connections.Update(ctx, ttsString(connection["id"]), patch); err != nil {
		return nil, fmt.Errorf("persist refreshed credentials: %w", err)
	}
	return patch, nil
}

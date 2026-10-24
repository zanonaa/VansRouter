package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
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
type refreshCall struct {
	done      chan struct{}
	patch     map[string]any
	err       error
	completed time.Time
}

func (t *TranslatorSend) refreshConnection(ctx context.Context, provider string, connection map[string]any) (map[string]any, error) {
	// Hash the secret rather than retaining a plaintext refresh token as a key.
	key := sha256.Sum256([]byte(provider + "\x00" + ttsString(connection["id"]) + "\x00" + ttsString(connection["refreshToken"])))
	t.refreshMu.Lock()
	if t.refreshCalls == nil {
		t.refreshCalls = map[[32]byte]*refreshCall{}
	}
	now := time.Now()
	for id, call := range t.refreshCalls {
		if !call.completed.IsZero() && now.Sub(call.completed) > 30*time.Second {
			delete(t.refreshCalls, id)
		}
	}
	if call := t.refreshCalls[key]; call != nil {
		t.refreshMu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-call.done:
			return copyRefreshPatch(call.patch), call.err
		}
	}
	call := &refreshCall{done: make(chan struct{})}
	t.refreshCalls[key] = call
	t.refreshMu.Unlock()
	patch, err := t.refreshConnectionOnce(ctx, provider, connection)
	t.refreshMu.Lock()
	call.patch, call.err, call.completed = copyRefreshPatch(patch), err, time.Now()
	// Failed requests can be retried; successful rotated tokens are reused by
	// lagging requests that still carry the old credentials for a short window.
	if err != nil || patch == nil {
		delete(t.refreshCalls, key)
	}
	close(call.done)
	t.refreshMu.Unlock()
	return patch, err
}

func copyRefreshPatch(patch map[string]any) map[string]any {
	if patch == nil {
		return nil
	}
	copy := make(map[string]any, len(patch))
	for k, v := range patch {
		copy[k] = v
	}
	return copy
}

func (t *TranslatorSend) refreshConnectionOnce(ctx context.Context, provider string, connection map[string]any) (map[string]any, error) {
	patch, err := oauthRefreshPatch(ctx, t.Client, provider, connection, t.GetPool)
	if err != nil || patch == nil {
		return nil, err
	}
	if _, err := t.Connections.Update(ctx, ttsString(connection["id"]), patch); err != nil {
		return nil, fmt.Errorf("persist refreshed credentials: %w", err)
	}
	return patch, nil
}

// oauthRefreshPatch executa o protocolo de refresh do provider e devolve o
// patch de credenciais, sem persistir. Compartilhado entre o envio do gateway e
// as telas de usage, que na origem também renovam antes de consultar a API.
func oauthRefreshPatch(ctx context.Context, base *http.Client, provider string, connection map[string]any, lookup connectionPoolLookup) (map[string]any, error) {
	refresh := ttsString(connection["refreshToken"])
	if refresh == "" {
		return nil, nil
	}
	switch provider {
	case "claude", "codex", "gemini-cli", "antigravity", "qwen", "grok-cli", "gcli", "xai":
	default:
		return nil, nil
	}
	// Grok CLI compartilha o cliente OAuth do xAI; os aliases usam o mesmo
	// endpoint/cliente da origem (tokenRefresh.js refreshXaiToken).
	configProvider := provider
	if provider == "gcli" || provider == "xai" {
		configProvider = "grok-cli"
	}
	config := providers.GetOAuthConfig(configProvider)
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
	requestCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, "POST", config.TokenURL, bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("Accept", "application/json")
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	client, relay, cleanup, err := connectionHTTP(requestCtx, metadata, lookup, base)
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
	return patch, nil
}

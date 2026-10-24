package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func NewQuotaAutoPing(db *sql.DB, client *http.Client) *QuotaAutoPing {
	connections := database.Connections{DB: db}
	refresh := &TranslatorSend{Connections: connections, Client: client, GetPool: (database.ProxyPools{DB: db}).Get}
	usage := &UsageContracts{Connections: connections, DB: db, Client: client}
	return &QuotaAutoPing{
		Settings:    func(ctx context.Context) (map[string]any, error) { return database.ReadAuthSettings(ctx, db) },
		Connections: connections.List, Update: connections.Update,
		Refresh: func(ctx context.Context, provider string, connection map[string]any) (map[string]any, error) {
			expiry, err := time.Parse(time.RFC3339Nano, ttsString(connection["expiresAt"]))
			if err != nil || time.Until(expiry) > 5*time.Minute {
				return connection, nil
			}
			patch, err := refresh.refreshConnection(ctx, provider, connection)
			if err != nil {
				return nil, err
			}
			if patch == nil {
				return nil, fmt.Errorf("unable to refresh quota ping credentials")
			}
			merged := map[string]any{}
			for k, v := range connection {
				merged[k] = v
			}
			for k, v := range patch {
				merged[k] = v
			}
			return merged, nil
		},
		Usage: func(ctx context.Context, provider string, connection map[string]any) (map[string]any, error) {
			request, err := http.NewRequestWithContext(ctx, "GET", "http://localhost/internal/quota-usage", nil)
			if err != nil {
				return nil, err
			}
			if provider == "claude" {
				return usage.claudeUsage(request, ttsString(connection["accessToken"])), nil
			}
			status, data, err := usage.resetJSON(ctx, "GET", "https://chatgpt.com/backend-api/wham/usage", map[string]string{"Authorization": "Bearer " + ttsString(connection["accessToken"]), "Accept": "application/json"}, nil)
			if err != nil {
				return nil, err
			}
			if status < 200 || status >= 300 {
				return nil, fmt.Errorf("quota usage API returned %d", status)
			}
			return normalizeCodexUsage(data), nil
		},
		Ping: func(ctx context.Context, provider string, connection map[string]any) (bool, error) {
			return sendQuotaPing(ctx, provider, connection, client, (database.ProxyPools{DB: db}).Get)
		},
	}
}

// sendQuotaPing is used only after the scheduler verifies the explicit per-account
// opt-in. It drains successful responses before marking the reset as pinged.
func sendQuotaPing(ctx context.Context, provider string, connection map[string]any, base *http.Client, lookup connectionPoolLookup) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	config := providers.TransportFor(provider)
	if config == nil {
		return false, fmt.Errorf("quota ping transport unavailable")
	}
	target := config.BaseURL
	headers := map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + ttsString(connection["accessToken"])}
	for k, v := range config.Headers {
		headers[k] = v
	}
	var payload map[string]any
	switch provider {
	case "claude":
		target += "?beta=true"
		payload = map[string]any{"model": "claude-haiku-4-5-20251001", "max_tokens": 1, "messages": []any{map[string]any{"role": "user", "content": "hi"}}}
	case "codex":
		payload = map[string]any{"model": "gpt-5.5", "input": []any{map[string]any{"type": "message", "role": "user", "content": []any{map[string]any{"type": "input_text", "text": "hi"}}}}, "instructions": "Reply with OK.", "reasoning": map[string]any{"effort": "none", "summary": "auto"}, "store": false, "stream": true}
		headers["session_id"] = ttsString(firstNonEmpty(connection["id"], "default"))
		headers["originator"] = "codex_cli_rs"
		metadata, _ := connection["providerSpecificData"].(map[string]any)
		if account := ttsString(firstNonEmpty(metadata["workspaceId"], metadata["chatgptAccountId"], metadata["accountId"])); account != "" {
			headers["ChatGPT-Account-ID"] = account
		}
	default:
		return false, fmt.Errorf("unsupported quota ping provider: %s", provider)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	request, err := http.NewRequestWithContext(ctx, "POST", target, bytes.NewReader(encoded))
	if err != nil {
		return false, err
	}
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	client, relay, cleanup, err := connectionHTTP(ctx, metadata, lookup, base)
	if err != nil {
		return false, err
	}
	defer cleanup()
	if err := rewriteConnectionRelay(request, relay); err != nil {
		return false, err
	}
	response, err := client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false, nil
	}
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		return false, err
	}
	return true, nil
}

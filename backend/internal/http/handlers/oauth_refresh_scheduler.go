package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Renova o access token ~5min antes do expiresAt persistido, reusando o
// refresh reativo existente (oauthRefreshPatch). Erros apenas registram; a
// via reativa (401→refresh→retry) continua como rede de segurança.
type OAuthRefreshScheduler struct {
	Connections NodeConnections
	Client      *http.Client
}

func StartOAuthRefreshScheduler(ctx context.Context, scheduler OAuthRefreshScheduler) {
	go func() {
		for {
			refreshDueOAuthConnections(ctx, scheduler)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	}()
}

func refreshDueOAuthConnections(ctx context.Context, scheduler OAuthRefreshScheduler) {
	all, err := scheduler.Connections.List(ctx, "", nil)
	if err != nil {
		slog.Warn("oauth refresh scheduler list failed", "error", err)
		return
	}
	now := time.Now()
	for _, connection := range all {
		provider := ttsString(connection["provider"])
		if !oauthRefreshSupported(provider) {
			continue
		}
		if ttsString(connection["refreshToken"]) == "" {
			continue
		}
		expiresAt := parseRefreshExpiry(connection)
		if expiresAt.IsZero() || now.Add(5*time.Minute).Before(expiresAt) {
			continue
		}
		refreshCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
		patch, refreshErr := oauthRefreshPatch(refreshCtx, scheduler.Client, provider, connection, nil)
		cancel()
		if refreshErr != nil || patch == nil {
			slog.Warn("oauth proactive refresh failed; reactive path remains", "provider", provider, "error", refreshErr)
			continue
		}
		if _, updateErr := scheduler.Connections.Update(ctx, ttsString(connection["id"]), patch); updateErr != nil {
			slog.Warn("oauth proactive refresh persist failed", "provider", provider, "error", updateErr)
		}
	}
}

func oauthRefreshSupported(provider string) bool {
	switch provider {
	case "claude", "codex", "gemini-cli", "antigravity", "qwen", "grok-cli", "gcli", "xai":
		return true
	}
	return false
}

func parseRefreshExpiry(connection map[string]any) time.Time {
	for _, key := range []string{"expiresAt", "tokenExpiresAt"} {
		if raw, ok := connection[key].(string); ok && raw != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
				return parsed
			}
		}
	}
	if seconds, ok := connection["expiresIn"].(float64); ok && seconds > 0 {
		if last, ok := connection["lastRefreshAt"].(string); ok && last != "" {
			if parsed, err := time.Parse(time.RFC3339Nano, last); err == nil {
				return parsed.Add(time.Duration(seconds * float64(time.Second)))
			}
		}
	}
	return time.Time{}
}

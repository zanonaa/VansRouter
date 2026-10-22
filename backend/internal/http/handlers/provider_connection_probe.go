package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// ConnectionProbe executes configured read-only probes and persists test state.
// Provider-specific refresh paths are not represented as successful probes.
type ConnectionProbe struct {
	Connections     usageConnectionStore
	Client          *http.Client
	GetPool         connectionPoolLookup
	ListConnections func(context.Context, string, *bool) ([]map[string]any, error)
}

func (p *ConnectionProbe) Test(ctx context.Context, id string) (map[string]any, error) {
	connection, err := p.Connections.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return map[string]any{"valid": false, "error": "Connection not found", "latencyMs": 0, "testedAt": time.Now().UTC().Format(time.RFC3339Nano)}, nil
	}
	provider := ttsString(connection["provider"])
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	target, token, authHeader, prefix := "", ttsString(firstNonEmpty(connection["accessToken"], connection["apiKey"])), "Authorization", "Bearer "
	if config := providers.Discovery(provider); config != nil {
		target = config.URL
		authHeader = config.AuthHeader
		prefix = config.AuthPrefix
	}
	switch provider {
	case "gemini-cli", "antigravity":
		target = "https://www.googleapis.com/oauth2/v1/userinfo?alt=json"
	case "github":
		target = "https://api.github.com/user"
	case "qoder":
		target = "https://openapi.qoder.sh/api/v1/userinfo"
	case "qoder-cn":
		target = "https://openapi.qoder.com.cn/api/v1/userinfo"
	case "gitlab":
		target = "https://gitlab.com/api/v4/user"
	}
	if strings.HasPrefix(provider, "openai-compatible-") {
		target = strings.TrimSuffix(ttsString(metadata["baseUrl"]), "/") + "/models"
		token = ttsString(connection["apiKey"])
	}
	started := time.Now()
	message := any(nil)
	valid := false
	if target == "" {
		message = "Provider test not supported"
	} else if token == "" {
		message = "No access token"
	} else {
		client, relay, cleanup, err := connectionHTTP(ctx, metadata, p.GetPool, p.Client)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		request, err := http.NewRequestWithContext(ctx, "GET", target, nil)
		if err != nil {
			return nil, err
		}
		request.Header.Set(authHeader, prefix+token)
		request.Header.Set("Accept", "application/json")
		request.Header.Set("User-Agent", "9Router")
		if err := rewriteConnectionRelay(request, relay); err != nil {
			return nil, err
		}
		response, err := client.Do(request)
		if err != nil {
			message = err.Error()
		} else {
			response.Body.Close()
			valid = response.StatusCode >= 200 && response.StatusCode < 300
			if !valid {
				switch response.StatusCode {
				case 401:
					message = "Token invalid or revoked"
				case 403:
					message = "Access denied"
				default:
					message = fmt.Sprintf("API returned %d", response.StatusCode)
				}
			}
		}
	}
	now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	patch := map[string]any{"testStatus": "error", "lastError": message, "lastErrorAt": now}
	if valid {
		patch["testStatus"] = "active"
		patch["lastErrorAt"] = nil
	}
	if _, err := p.Connections.Update(ctx, id, patch); err != nil {
		return nil, err
	}
	return map[string]any{"valid": valid, "error": message, "refreshed": false, "latencyMs": time.Since(started).Milliseconds(), "testedAt": now}, nil
}
func (p *ConnectionProbe) Post(w http.ResponseWriter, r *http.Request) {
	result, err := p.Test(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": "Test failed"})
		return
	}
	if result["error"] == "Connection not found" {
		nodeReply(w, 404, map[string]any{"error": "Connection not found"})
		return
	}
	nodeReply(w, 200, map[string]any{"valid": result["valid"], "error": result["error"], "refreshed": result["refreshed"]})
}

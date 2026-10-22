package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// GCPProjects retains refresh-before-list behavior for the two Google OAuth
// providers. Store/HTTP injection keeps tests independent of credentials/network.
type GCPProjects struct {
	Connections usageConnectionStore
	Client      *http.Client
	GetPool     connectionPoolLookup
}

func (g *GCPProjects) client() *http.Client {
	if g.Client != nil {
		return g.Client
	}
	return http.DefaultClient
}
func (g *GCPProjects) refresh(ctx context.Context, connection map[string]any) (string, error) {
	access := ttsString(connection["accessToken"])
	expiry, err := time.Parse(time.RFC3339Nano, ttsString(connection["expiresAt"]))
	expired := connection["expiresAt"] == nil || err == nil && expiry.Before(time.Now())
	refresh := ttsString(connection["refreshToken"])
	if !expired || refresh == "" {
		return access, nil
	}
	config := providers.GetOAuthConfig(ttsString(connection["provider"]))
	if config == nil || config.TokenURL == "" {
		return access, nil
	}
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refresh}, "client_id": {config.ClientID}, "client_secret": {config.ClientSecret}}
	request, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := g.client().Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return access, nil
	}
	var tokens map[string]any
	if err := json.NewDecoder(response.Body).Decode(&tokens); err != nil {
		return "", err
	}
	if newAccess := ttsString(tokens["access_token"]); newAccess != "" {
		patch := map[string]any{"accessToken": newAccess}
		if value := ttsString(tokens["refresh_token"]); value != "" {
			patch["refreshToken"] = value
		}
		if seconds := pxpipeNum(tokens["expires_in"]); seconds > 0 {
			patch["expiresAt"] = time.Now().Add(time.Duration(seconds * float64(time.Second))).UTC().Format("2006-01-02T15:04:05.000Z")
		}
		if _, err := g.Connections.Update(ctx, ttsString(connection["id"]), patch); err != nil {
			return "", err
		}
		access = newAccess
	}
	return access, nil
}

func (g *GCPProjects) Get(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]any{"error": "Failed to fetch GCP projects"}) }
	connection, err := g.Connections.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		fail()
		return
	}
	if connection == nil {
		nodeReply(w, 404, map[string]any{"error": "Connection not found"})
		return
	}
	if connection["provider"] != "gemini-cli" && connection["provider"] != "antigravity" {
		nodeReply(w, 400, map[string]any{"error": "Provider not supported for GCP projects"})
		return
	}
	token, err := g.refresh(r.Context(), connection)
	if err != nil {
		fail()
		return
	}
	if token == "" {
		nodeReply(w, 401, map[string]any{"error": "No valid access token available"})
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), "GET", "https://cloudresourcemanager.googleapis.com/v1/projects", nil)
	if err != nil {
		fail()
		return
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/json")
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	client, relay, cleanup, err := connectionHTTP(r.Context(), metadata, g.GetPool, g.Client)
	if err != nil {
		fail()
		return
	}
	defer cleanup()
	if err := rewriteConnectionRelay(request, relay); err != nil {
		fail()
		return
	}
	response, err := client.Do(request)
	if err != nil {
		fail()
		return
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, _ := io.ReadAll(response.Body)
		nodeReply(w, response.StatusCode, map[string]any{"error": fmt.Sprintf("Google API error: %d - %s", response.StatusCode, body)})
		return
	}
	var data struct {
		Projects []map[string]any `json:"projects"`
	}
	if err := json.NewDecoder(response.Body).Decode(&data); err != nil {
		fail()
		return
	}
	projects := []map[string]any{}
	for _, project := range data.Projects {
		entry := map[string]any{}
		if value, exists := project["projectId"]; exists {
			entry["id"] = value
		}
		if value, exists := project["name"]; exists {
			entry["name"] = value
		}
		projects = append(projects, entry)
	}
	nodeReply(w, 200, map[string]any{"projects": projects})
}

package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type usageConnectionStore interface {
	Get(context.Context, string) (map[string]any, error)
	Update(context.Context, string, map[string]any) (map[string]any, error)
}
type UsageContracts struct {
	Connections usageConnectionStore
	DB          *sql.DB
	Client      *http.Client
}

func (u *UsageContracts) Stats(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}
	valid := map[string]bool{"today": true, "24h": true, "7d": true, "30d": true, "60d": true, "all": true}
	if !valid[period] {
		nodeReply(w, 400, map[string]any{"error": "Invalid period"})
		return
	}
	days := map[string]int{"today": 1, "24h": 1, "7d": 7, "30d": 30, "60d": 60}[period]
	start := time.Time{}
	if days > 0 {
		start = time.Now().AddDate(0, 0, -days)
	}
	rows, err := u.DB.QueryContext(r.Context(), "SELECT COUNT(*),COALESCE(SUM(promptTokens),0),COALESCE(SUM(completionTokens),0),COALESCE(SUM(cost),0) FROM usageHistory WHERE (?='' OR timestamp>=?)", start.UTC().Format(time.RFC3339Nano), start.UTC().Format(time.RFC3339Nano))
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": "Failed to fetch usage stats"})
		return
	}
	defer rows.Close()
	var count, prompt, completion int64
	var cost float64
	if rows.Next() {
		rows.Scan(&count, &prompt, &completion, &cost)
	}
	nodeReply(w, 200, map[string]any{"period": period, "totalRequests": count, "totalPromptTokens": prompt, "totalCompletionTokens": completion, "totalCost": cost})
}
func (u *UsageContracts) connection(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	item, err := u.Connections.Get(r.Context(), chi.URLParam(r, "connectionId"))
	if err != nil || item == nil {
		nodeReply(w, 404, map[string]any{"error": "Connection not found"})
		return nil, false
	}
	return item, true
}
func (u *UsageContracts) Get(w http.ResponseWriter, r *http.Request) {
	connection, ok := u.connection(w, r)
	if !ok {
		return
	}
	auth := ttsString(connection["authType"])
	eligible := auth == "oauth" || (auth == "apikey" || auth == "api_key") && usageAPIKeyProviders[ttsString(connection["provider"])]
	if !eligible {
		nodeReply(w, 200, map[string]any{"message": "Usage not available for this connection"})
		return
	}
	nodeReply(w, 501, map[string]any{"error": "Provider usage executor not ported", "provider": connection["provider"]})
}
func (u *UsageContracts) ClaudeReset(w http.ResponseWriter, r *http.Request) {
	connection, ok := u.connection(w, r)
	if !ok {
		return
	}
	if connection["provider"] != "claude" || connection["authType"] != "oauth" {
		nodeReply(w, 400, map[string]any{"error": "Limit reset is only available for Claude OAuth connections."})
		return
	}
	body, _ := poolInput(r)
	grant := ttsString(body["grantId"])
	request, err := http.NewRequestWithContext(r.Context(), "POST", "https://api.anthropic.com/api/oauth/usage/reset", strings.NewReader(`{"grantId":"`+grant+`"}`))
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	request.Header.Set("Authorization", "Bearer "+ttsString(connection["accessToken"]))
	request.Header.Set("Content-Type", "application/json")
	client := u.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	defer response.Body.Close()
	nodeReply(w, response.StatusCode, map[string]any{"ok": response.StatusCode < 300, "status": response.StatusCode})
}
func (u *UsageContracts) CodexCredits(w http.ResponseWriter, r *http.Request) {
	connection, ok := u.connection(w, r)
	if !ok {
		return
	}
	if connection["provider"] != "codex" || connection["authType"] != "oauth" && connection["authType"] != "access_token" {
		nodeReply(w, 400, map[string]any{"error": "Codex reset credits require an OAuth or access-token connection."})
		return
	}
	method := "GET"
	if r.Method == "POST" {
		method = "POST"
	}
	request, err := http.NewRequestWithContext(r.Context(), method, "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits", nil)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	request.Header.Set("Authorization", "Bearer "+ttsString(connection["accessToken"]))
	client := u.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	defer response.Body.Close()
	nodeReply(w, response.StatusCode, map[string]any{"status": response.StatusCode})
}

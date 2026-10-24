package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type usageConnectionStore interface {
	Get(context.Context, string) (map[string]any, error)
	Update(context.Context, string, map[string]any) (map[string]any, error)
}
type UsageContracts struct {
	Connections    usageConnectionStore
	DB             *sql.DB
	LiveUsage      *UsageLive
	Client         *http.Client
	claudeMu       sync.Mutex
	claudeCache    map[[32]byte]*claudeUsageCacheEntry
	claudeCooldown map[[32]byte]time.Time
	weeklyUsage    map[[32]byte]*weeklyUsageEntry
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
	stats, err := u.usageStats(r, period)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": "Failed to fetch usage stats"})
		return
	}
	nodeReply(w, 200, stats)
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
	u.providerUsage(w, r, connection)
}
func (u *UsageContracts) ClaudeReset(w http.ResponseWriter, r *http.Request) {
	u.claudeReset(w, r)
}
func (u *UsageContracts) CodexCredits(w http.ResponseWriter, r *http.Request) {
	u.codexCredits(w, r)
}

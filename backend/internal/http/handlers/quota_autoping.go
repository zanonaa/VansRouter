package handlers

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// QuotaAutoPing only targets active OAuth accounts explicitly opted into settings.
// Tick's injected dependencies make scheduling policy testable without paid calls.
type QuotaAutoPing struct {
	Settings    func(context.Context) (map[string]any, error)
	Connections func(context.Context, string, *bool) ([]map[string]any, error)
	Refresh     func(context.Context, string, map[string]any) (map[string]any, error)
	Usage       func(context.Context, string, map[string]any) (map[string]any, error)
	Ping        func(context.Context, string, map[string]any) (bool, error)
	Update      func(context.Context, string, map[string]any) (map[string]any, error)
	Now         func() time.Time
	mu          sync.Mutex
	running     bool
	resets      map[string]time.Time
	failures    map[string]time.Time
}

func (q *QuotaAutoPing) now() time.Time {
	if q.Now != nil {
		return q.Now()
	}
	return time.Now()
}
func quotaNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, !math.IsNaN(v) && !math.IsInf(v, 0)
	case int:
		return float64(v), true
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
	}
	return 0, false
}
func exhaustedQuota(value any) bool {
	quota, _ := value.(map[string]any)
	if quota["unlimited"] == true {
		return false
	}
	if remaining, ok := quotaNumber(quota["remaining"]); ok {
		return remaining <= 0
	}
	used, hasUsed := quotaNumber(quota["used"])
	total, hasTotal := quotaNumber(quota["total"])
	return hasUsed && hasTotal && total > 0 && used >= total
}
func (q *QuotaAutoPing) Tick(ctx context.Context) error {
	q.mu.Lock()
	if q.running {
		q.mu.Unlock()
		return nil
	}
	q.running = true
	if q.resets == nil {
		q.resets = map[string]time.Time{}
	}
	if q.failures == nil {
		q.failures = map[string]time.Time{}
	}
	q.mu.Unlock()
	defer func() { q.mu.Lock(); q.running = false; q.mu.Unlock() }()
	settings, err := q.Settings(ctx)
	if err != nil {
		return err
	}
	for _, provider := range []string{"claude", "codex"} {
		config, _ := settings[provider+"AutoPing"].(map[string]any)
		enabled, _ := config["connections"].(map[string]any)
		if len(enabled) == 0 {
			continue
		}
		active := true
		connections, err := q.Connections(ctx, provider, &active)
		if err != nil {
			return err
		}
		for _, connection := range connections {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			id := ttsString(connection["id"])
			if connection["authType"] != "oauth" || enabled[id] != true || connection["isActive"] == false {
				continue
			}
			key := provider + ":" + id
			now := q.now()
			cached := q.resets[key]
			if provider == "claude" && !cached.IsZero() && now.Before(cached.Add(-5*time.Minute)) {
				continue
			}
			if failed := q.failures[key]; !failed.IsZero() && now.Sub(failed) < 15*time.Minute {
				continue
			}
			if err := q.pingAccount(ctx, provider, key, cached, connection); err != nil {
				q.failures[key] = q.now()
			}
		}
	}
	return nil
}
func (q *QuotaAutoPing) pingAccount(ctx context.Context, provider, key string, cached time.Time, connection map[string]any) error {
	var err error
	if q.Refresh != nil {
		connection, err = q.Refresh(ctx, provider, connection)
		if err != nil {
			return err
		}
	}
	usage, err := q.Usage(ctx, provider, connection)
	if err != nil {
		return err
	}
	quotas, _ := usage["quotas"].(map[string]any)
	session := "session (5h)"
	if provider == "codex" {
		session = "session"
	}
	quota, _ := quotas[session].(map[string]any)
	reset, err := time.Parse(time.RFC3339Nano, ttsString(quota["resetAt"]))
	if err != nil {
		return nil
	}
	q.resets[key] = reset
	if exhaustedQuota(quota) {
		return nil
	}
	now := q.now()
	if provider == "codex" {
		for name, value := range quotas {
			if name != session && !strings.Contains(strings.ToLower(name), "session") && exhaustedQuota(value) {
				return nil
			}
		}
		if cached.IsZero() || reset.Sub(cached) < 30*time.Second {
			return nil
		}
		if last, err := time.Parse(time.RFC3339Nano, ttsString(connection["lastPingAt"])); err == nil && now.Sub(last) < 10*time.Minute {
			return nil
		}
	} else if now.Before(reset.Add(-5 * time.Second)) {
		return nil
	}
	resetKey := reset.UTC().Truncate(time.Minute).Format("2006-01-02T15:04:05.000Z")
	previous := ttsString(connection["lastPingedResetKey"])
	if previous == "" {
		if last, err := time.Parse(time.RFC3339Nano, ttsString(connection["lastPingedResetAt"])); err == nil {
			previous = last.UTC().Truncate(time.Minute).Format("2006-01-02T15:04:05.000Z")
		}
	}
	if previous == resetKey {
		return nil
	}
	ok, err := q.Ping(ctx, provider, connection)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("quota ping rejected")
	}
	_, err = q.Update(ctx, ttsString(connection["id"]), map[string]any{"lastPingedResetAt": reset.UTC().Format("2006-01-02T15:04:05.000Z"), "lastPingedResetKey": resetKey, "lastPingAt": now.UTC().Format("2006-01-02T15:04:05.000Z"), "updatedAt": now.UTC().Format("2006-01-02T15:04:05.000Z")})
	if err == nil {
		delete(q.failures, key)
	}
	return err
}
func (q *QuotaAutoPing) Run(ctx context.Context) {
	q.Tick(ctx)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.Tick(ctx)
		}
	}
}

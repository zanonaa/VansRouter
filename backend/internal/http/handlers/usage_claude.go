package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func normalizeClaudeUsage(data map[string]any) map[string]any {
	quotas := map[string]any{}
	add := func(label string, window map[string]any) {
		used, ok := window["utilization"].(float64)
		if !ok {
			return
		}
		remaining := math.Max(0, 100-used)
		quotas[label] = map[string]any{"used": used, "total": 100, "remaining": remaining, "remainingPercentage": remaining, "resetAt": usageResetDate(window["resets_at"]), "unlimited": false}
	}
	for key, value := range data {
		window, _ := value.(map[string]any)
		switch {
		case key == "five_hour":
			add("session (5h)", window)
		case key == "seven_day":
			add("weekly (7d)", window)
		case strings.HasPrefix(key, "seven_day_"):
			add("weekly "+strings.TrimPrefix(key, "seven_day_")+" (7d)", window)
		}
	}
	limits, _ := data["limits"].([]any)
	for _, raw := range limits {
		limit, _ := raw.(map[string]any)
		if limit["kind"] != "weekly_scoped" {
			continue
		}
		scope, _ := limit["scope"].(map[string]any)
		model, _ := scope["model"].(map[string]any)
		name := strings.ToLower(strings.TrimSpace(ttsString(model["display_name"])))
		percent, ok := limit["percent"].(float64)
		if name == "" || !ok {
			continue
		}
		add("weekly "+name+" (7d)", map[string]any{"utilization": math.Max(0, math.Min(100, percent)), "resets_at": limit["resets_at"]})
	}
	return map[string]any{"plan": "Claude Code", "extraUsage": data["extra_usage"], "resetCredits": parseClaudeGrants(data["cedar_ember"]), "quotas": quotas}
}
func parseClaudeGrants(value any) any {
	block, _ := value.(map[string]any)
	grants, ok := block["grants"].([]any)
	if !nodeTruthy(block["eligible"]) || !ok {
		return nil
	}
	available := float64(0)
	var next map[string]any
	all := []any{}
	for _, raw := range grants {
		grant, _ := raw.(map[string]any)
		if !nodeTruthy(grant["id"]) {
			continue
		}
		count := usageNumber(grant["resets_left"])
		if !nodeTruthy(grant["paused"]) && count > 0 {
			available += count
			if next == nil || grant["id"] == block["next_grant_id"] {
				next = grant
			}
		}
		clears, _ := grant["clears"].([]any)
		if clears == nil {
			clears = []any{}
		}
		label := ttsString(grant["label"])
		all = append(all, map[string]any{"id": grant["id"], "label": label, "resetsLeft": count, "resetsTotal": usageNumber(grant["resets_total"]), "startsAt": grant["starts_at"], "endsAt": grant["ends_at"], "clears": clears, "paused": grant["paused"] == true, "usableNow": grant["usable_now"] == true, "useRequiresLimit": grant["use_requires_limit"] != false})
	}
	clears := any([]any{})
	if nodeTruthy(next["clears"]) {
		clears = next["clears"]
	}
	return map[string]any{"availableCount": available, "nextGrantId": next["id"], "expiresAt": next["ends_at"], "clears": clears, "cooldownUntil": block["cooldown_until"], "weeklyResetsAt": block["weekly_resets_at"], "grants": all}
}

type claudeUsageCacheEntry struct {
	done    chan struct{}
	result  map[string]any
	expires time.Time
}

func cloneClaudeUsage(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	raw, _ := json.Marshal(value)
	var result map[string]any
	json.Unmarshal(raw, &result)
	return result
}

func (u *UsageContracts) claudeUsage(r *http.Request, token string) map[string]any {
	if token == "" {
		return u.claudeUsageRaw(r, token)
	}
	key := sha256.Sum256([]byte(token))
	force := r.URL.Query().Get("force") == "true"
	u.claudeMu.Lock()
	if u.claudeCache == nil {
		u.claudeCache = map[[32]byte]*claudeUsageCacheEntry{}
	}
	current := u.claudeCache[key]
	if !force && current != nil {
		if current.done != nil {
			done := current.done
			u.claudeMu.Unlock()
			select {
			case <-r.Context().Done():
				return map[string]any{"message": "Claude connected. Unable to fetch usage: " + r.Context().Err().Error()}
			case <-done:
				return cloneClaudeUsage(current.result)
			}
		}
		if current.expires.After(time.Now()) {
			result := cloneClaudeUsage(current.result)
			u.claudeMu.Unlock()
			return result
		}
	}
	var stale map[string]any
	if !force && current != nil {
		stale = current.result
	}
	entry := &claudeUsageCacheEntry{done: make(chan struct{})}
	u.claudeCache[key] = entry
	u.claudeMu.Unlock()
	result := u.claudeUsageRaw(r, token)
	quotas, _ := result["quotas"].(map[string]any)
	if len(quotas) == 0 && stale != nil {
		result = stale
	}
	u.claudeMu.Lock()
	entry.result = cloneClaudeUsage(result)
	entry.expires = time.Now().Add(5 * time.Minute)
	close(entry.done)
	entry.done = nil
	if active := u.claudeCache[key]; active == entry {
		quotas, _ := result["quotas"].(map[string]any)
		if len(quotas) == 0 {
			delete(u.claudeCache, key)
		}
	}
	for id, cached := range u.claudeCache {
		if cached.done == nil && cached.expires.Before(time.Now()) {
			delete(u.claudeCache, id)
		}
	}
	if len(u.claudeCache) > 100 {
		for id, cached := range u.claudeCache {
			if id != key && cached.done == nil {
				delete(u.claudeCache, id)
				if len(u.claudeCache) <= 100 {
					break
				}
			}
		}
	}
	u.claudeMu.Unlock()
	return cloneClaudeUsage(result)
}

func (u *UsageContracts) claudeUsageRaw(r *http.Request, token string) map[string]any {
	key := sha256.Sum256([]byte(token))
	u.claudeMu.Lock()
	cooling := u.claudeCooldown[key].After(time.Now())
	u.claudeMu.Unlock()
	if cooling {
		return u.claudeLegacyUsage(r, token)
	}
	headers := map[string]string{"Authorization": "Bearer " + token, "anthropic-beta": "oauth-2025-04-20", "anthropic-version": "2023-06-01", "User-Agent": "claude-cli/2.1.280 (external, cli)"}
	status, data, err := u.resetJSON(r.Context(), "GET", "https://api.anthropic.com/api/oauth/usage?cedar_ember=1", headers, nil)
	if err != nil {
		return map[string]any{"message": "Claude connected. Unable to fetch usage: " + err.Error()}
	}
	if status >= 200 && status < 300 {
		return normalizeClaudeUsage(data)
	}
	if status == 429 {
		u.claudeMu.Lock()
		if u.claudeCooldown == nil {
			u.claudeCooldown = map[[32]byte]time.Time{}
		}
		u.claudeCooldown[key] = time.Now().Add(3 * time.Minute)
		u.claudeMu.Unlock()
	}
	return u.claudeLegacyUsage(r, token)
}
func (u *UsageContracts) claudeLegacyUsage(r *http.Request, token string) map[string]any {
	headers := map[string]string{"Authorization": "Bearer " + token, "anthropic-version": "2023-06-01"}
	status, settings, err := u.resetJSON(r.Context(), "GET", "https://api.anthropic.com/v1/settings", headers, nil)
	if err != nil {
		return map[string]any{"message": "Claude connected. Unable to fetch usage: " + err.Error()}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"message": "Claude connected. Usage API requires admin permissions."}
	}
	result := map[string]any{"plan": firstNonEmpty(settings["plan"], "Unknown")}
	if organization, ok := settings["organization_name"]; ok {
		result["organization"] = organization
	}
	if org := ttsString(settings["organization_id"]); org != "" {
		status, usage, err := u.resetJSON(r.Context(), "GET", "https://api.anthropic.com/v1/organizations/"+url.PathEscape(org)+"/usage", headers, nil)
		if err != nil {
			return map[string]any{"message": "Claude connected. Unable to fetch usage: " + err.Error()}
		}
		if status >= 200 && status < 300 {
			result["quotas"] = usage
			return result
		}
	}
	result["message"] = "Claude connected. Usage details require admin access."
	return result
}

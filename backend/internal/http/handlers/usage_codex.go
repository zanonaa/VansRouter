package handlers

import (
	"math"
	"strconv"
	"strings"
	"time"
)

func usageFirstPresent(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func usageResetDate(value any) any {
	if !nodeTruthy(value) {
		return nil
	}
	if text, ok := value.(string); ok {
		if number, err := strconv.ParseFloat(text, 64); err == nil {
			return usageResetDate(number)
		}
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			return parsed.UTC().Format("2006-01-02T15:04:05.000Z")
		}
		return nil
	}
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
		return nil
	}
	if number < 1e12 {
		number *= 1000
	}
	return time.UnixMilli(int64(number)).UTC().Format("2006-01-02T15:04:05.000Z")
}
func codexRateBody(value any) map[string]any {
	snapshot, _ := value.(map[string]any)
	if body, ok := snapshot["rate_limit"].(map[string]any); ok {
		return body
	}
	return snapshot
}
func codexSpecialLimit(data map[string]any, kind string) any {
	keys := []string{"code_review_rate_limit", "review_rate_limit"}
	byKeys := []string{"code_review", "codex_review", "review"}
	if kind == "spark" {
		keys = []string{"spark_rate_limit", "gpt_5_3_codex_spark_rate_limit"}
		byKeys = []string{"gpt-5.3-codex-spark", "gpt_5_3_codex_spark", "spark"}
	}
	for _, key := range keys {
		if nodeTruthy(data[key]) {
			return data[key]
		}
	}
	byID, _ := data["rate_limits_by_limit_id"].(map[string]any)
	for _, key := range byKeys {
		if nodeTruthy(byID[key]) {
			return byID[key]
		}
	}
	additional, _ := data["additional_rate_limits"].([]any)
	for _, raw := range additional {
		entry, _ := raw.(map[string]any)
		id := strings.ToLower(ttsString(firstNonEmpty(entry["limit_name"], entry["metered_feature"], entry["id"])))
		if strings.Contains(id, kind) {
			return entry
		}
	}
	return nil
}
func normalizeCodexUsage(data map[string]any) map[string]any {
	byID, _ := data["rate_limits_by_limit_id"].(map[string]any)
	normal := firstNonEmpty(data["rate_limit"], data["rate_limits"], byID["codex"])
	review, spark := codexSpecialLimit(data, "review"), codexSpecialLimit(data, "spark")
	quotas := map[string]any{}
	for _, entry := range []struct {
		prefix   string
		snapshot any
	}{{"", normal}, {"review_", review}, {"spark_", spark}} {
		snapshot, _ := entry.snapshot.(map[string]any)
		rate := codexRateBody(snapshot)
		if rate == nil {
			continue
		}
		for _, window := range []struct {
			name  string
			value any
		}{{"session", firstNonEmpty(rate["primary_window"], rate["primary"], snapshot["primary_window"], snapshot["primary"])}, {"weekly", firstNonEmpty(rate["secondary_window"], rate["secondary"], snapshot["secondary_window"], snapshot["secondary"])}} {
			value, ok := window.value.(map[string]any)
			if !ok {
				continue
			}
			used := math.Max(0, math.Min(100, usageNumber(usageFirstPresent(value["used_percent"], value["percent_used"]))))
			quotas[entry.prefix+window.name] = map[string]any{"used": used, "total": 100, "remaining": math.Max(0, 100-used), "resetAt": usageResetDate(usageFirstPresent(value["reset_at"], value["resets_at"], value["resetAt"])), "unlimited": false}
		}
	}
	summary, _ := data["summary"].(map[string]any)
	credits, _ := data["rate_limit_reset_credits"].(map[string]any)
	return map[string]any{"plan": firstNonEmpty(data["plan_type"], summary["plan"], "unknown"), "limitReached": nodeTruthy(codexRateBody(normal)["limit_reached"]), "reviewLimitReached": nodeTruthy(codexRateBody(review)["limit_reached"]), "sparkLimitReached": nodeTruthy(codexRateBody(spark)["limit_reached"]), "resetCredits": map[string]any{"availableCount": math.Max(0, usageNumber(credits["available_count"]))}, "quotas": quotas}
}

package handlers

import (
	"math"
	"strconv"
	"strings"
	"time"
)

func usageNumber(value any) float64 {
	if text, ok := value.(string); ok {
		number, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err == nil {
			return number
		}
	}
	return pxpipeNum(value)
}

func normalizeDeepSeekUsage(data map[string]any) map[string]any {
	if data == nil {
		return map[string]any{"message": "DeepSeek balance response was not JSON."}
	}
	entries, _ := data["balance_infos"].([]any)
	quotas := map[string]any{}
	for _, raw := range entries {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		currency := strings.ToUpper(ttsString(item["currency"]))
		if currency == "" {
			continue
		}
		total := math.Max(0, usageNumber(firstNonEmpty(item["total_balance"], item["totalBalance"])))
		remaining := 0.0
		if total > 0 {
			remaining = 100
		}
		quotas["Balance ("+currency+")"] = map[string]any{"used": 0, "total": total, "remainingPercentage": remaining, "resetAt": nil, "unlimited": false, "isCreditBalance": true, "currency": currency}
	}
	if len(quotas) == 0 {
		return map[string]any{"plan": "DeepSeek", "message": "DeepSeek connected. No balance data returned."}
	}
	plan := "DeepSeek (Insufficient Balance)"
	if data["is_available"] == true || data["isAvailable"] == true {
		plan = "DeepSeek"
	}
	return map[string]any{"plan": plan, "quotas": quotas}
}

func addUsageMonths(value time.Time, months int) time.Time {
	year, month := value.Year(), int(value.Month())+months
	year += (month - 1) / 12
	month = (month-1)%12 + 1
	day := value.Day()
	last := time.Date(year, time.Month(month)+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > last {
		day = last
	}
	return time.Date(year, time.Month(month), day, value.Hour(), value.Minute(), value.Second(), 0, time.UTC)
}

func normalizeOllamaUsage(data, profile map[string]any) map[string]any {
	if data == nil {
		return map[string]any{"message": "Ollama Cloud usage response was not JSON."}
	}
	planRaw := strings.TrimSpace(ttsString(profile["Plan"]))
	plan := "Ollama Cloud"
	if planRaw != "" {
		plan = strings.ToUpper(planRaw[:1]) + strings.ToLower(planRaw[1:])
	}
	var monthlyReset any
	if strings.EqualFold(planRaw, "free") {
		if created, err := time.Parse(time.RFC3339Nano, ttsString(profile["CreatedAt"])); err == nil {
			now := time.Now().UTC()
			elapsed := (now.Year()-created.Year())*12 + int(now.Month()) - int(created.Month())
			for months := max(0, elapsed); months <= elapsed+1; months++ {
				candidate := addUsageMonths(created.UTC(), months)
				if candidate.After(now) {
					monthlyReset = candidate.Format(time.RFC3339Nano)
					break
				}
			}
		}
	}
	limits, _ := data["limits"].(map[string]any)
	labels := []struct{ key, label string }{{"session", "Session (5h)"}, {"weekly", "Weekly (7d)"}, {"monthly", "Monthly"}}
	quotas := map[string]any{}
	for _, item := range labels {
		window, _ := limits[item.key].(map[string]any)
		if window == nil || window["usage"] == nil {
			continue
		}
		ratio := math.Min(1, math.Max(0, usageNumber(window["usage"])))
		used := math.Round(ratio * 100)
		entry := map[string]any{"used": used, "total": 100, "remainingPercentage": 100 - used, "resetAt": nil, "unlimited": false}
		if item.key == "monthly" {
			entry["resetAt"] = monthlyReset
		}
		quotas[item.label] = entry
	}
	if len(quotas) == 0 {
		return map[string]any{"plan": plan, "message": "Ollama Cloud connected. No usage limits reported.", "quotas": quotas}
	}
	return map[string]any{"plan": plan, "quotas": quotas}
}

package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

// usageStats supplies the complete dashboard contract, not just the totals.
func (u *UsageContracts) usageStats(r *http.Request, period string) (map[string]any, error) {
	ctx := r.Context()
	now := time.Now()
	start := ""
	days := map[string]int{"7d": 7, "30d": 30, "60d": 60}[period]
	if period == "today" {
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).UTC().Format(time.RFC3339)
	}
	if period == "24h" {
		start = now.Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	}
	connections := map[string]string{}
	if items, err := (database.Connections{DB: u.DB}).List(ctx, "", nil); err == nil {
		for _, item := range items {
			connections[ttsString(item["id"])] = firstNonEmptyString(item["name"], item["email"], item["id"])
		}
	}
	keyNames := map[string]string{}
	rows, err := u.DB.QueryContext(ctx, `SELECT key,name FROM apiKeys`)
	if err == nil {
		for rows.Next() {
			var key string
			var name any
			if rows.Scan(&key, &name) == nil {
				keyNames[key] = ttsString(name)
			}
		}
		rows.Close()
	}
	stats := map[string]any{"period": period, "totalRequests": float64(0), "totalPromptTokens": float64(0), "totalCompletionTokens": float64(0), "totalCachedTokens": float64(0), "totalCost": float64(0)}
	for _, group := range []string{"byProvider", "byModel", "byAccount", "byApiKey", "byEndpoint"} {
		stats[group] = map[string]any{}
	}
	add := func(group, key string, values map[string]any, metadata map[string]any) {
		bucket := stats[group].(map[string]any)
		entry, _ := bucket[key].(map[string]any)
		if entry == nil {
			entry = map[string]any{}
			bucket[key] = entry
		}
		for _, field := range []string{"requests", "promptTokens", "completionTokens", "cachedTokens", "cost"} {
			entry[field] = pxpipeNum(entry[field]) + pxpipeNum(values[field])
		}
		for k, v := range metadata {
			entry[k] = v
		}
	}
	consume := func(day map[string]any, date string) {
		for total, field := range map[string]string{"totalRequests": "requests", "totalPromptTokens": "promptTokens", "totalCompletionTokens": "completionTokens", "totalCachedTokens": "cachedTokens", "totalCost": "cost"} {
			stats[total] = pxpipeNum(stats[total]) + pxpipeNum(day[field])
		}
		for _, group := range []string{"byProvider", "byModel", "byAccount", "byApiKey", "byEndpoint"} {
			entries, _ := day[group].(map[string]any)
			for key, raw := range entries {
				values, _ := raw.(map[string]any)
				model, provider := ttsString(values["rawModel"]), ttsString(values["provider"])
				meta := map[string]any{"rawModel": model, "provider": provider, "lastUsed": date}
				displayKey := key
				switch group {
				case "byModel":
					if model == "" {
						parts := strings.SplitN(key, "|", 2)
						model = parts[0]
						if len(parts) > 1 {
							provider = parts[1]
						}
						meta["rawModel"] = model
						meta["provider"] = provider
					}
					displayKey = model + " (" + provider + ")"
				case "byAccount":
					meta["connectionId"] = key
					meta["accountName"] = firstNonEmptyString(connections[key], key)
					displayKey = model + " (" + provider + " - " + ttsString(meta["accountName"]) + ")"
				case "byApiKey":
					apiKey := ttsString(values["apiKey"])
					masked := maskUsageKey(apiKey)
					if strings.Contains(apiKey, "***") {
						masked = apiKey
					}
					meta["apiKeyMasked"] = masked
					meta["apiKeyKey"] = firstNonEmptyString(masked, "local-no-key")
					meta["keyName"] = firstNonEmptyString(keyNames[apiKey], masked, "Local (No API Key)")
					displayKey = ttsString(meta["apiKeyKey"]) + "|" + model + "|" + provider
				case "byEndpoint":
					meta["endpoint"] = values["endpoint"]
				}
				add(group, displayKey, values, meta)
			}
		}
	}
	if period != "today" && period != "24h" {
		cutoff := ""
		if days > 0 {
			cutoff = now.AddDate(0, 0, -days+1).Format("2006-01-02")
		}
		rows, err := u.DB.QueryContext(ctx, `SELECT dateKey,data FROM usageDaily WHERE dateKey>=? ORDER BY dateKey`, cutoff)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var date, raw string
			if err = rows.Scan(&date, &raw); err != nil {
				rows.Close()
				return nil, err
			}
			var day map[string]any
			json.Unmarshal([]byte(raw), &day)
			consume(day, date)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	} else {
		history, err := (database.Usage{DB: u.DB}).History(ctx, "", "", start, "")
		if err != nil {
			return nil, err
		}
		for _, entry := range history {
			tokens, _ := entry["tokens"].(map[string]any)
			day := map[string]any{"requests": float64(1), "promptTokens": pxpipeNum(firstNonEmpty(tokens["prompt_tokens"], tokens["input_tokens"])), "completionTokens": pxpipeNum(firstNonEmpty(tokens["completion_tokens"], tokens["output_tokens"])), "cachedTokens": pxpipeNum(firstNonEmpty(tokens["cached_tokens"], tokens["cache_read_input_tokens"])), "cost": entry["cost"]}
			model, provider, connection := ttsString(entry["model"]), ttsString(entry["provider"]), ttsString(entry["connectionId"])
			values := map[string]any{}
			for k, v := range day {
				values[k] = v
			}
			values["rawModel"] = model
			values["provider"] = provider
			values["endpoint"] = entry["endpoint"]
			values["apiKey"] = entry["apiKeyMasked"]
			for group, key := range map[string]string{"byProvider": provider, "byModel": model + "|" + provider, "byAccount": connection, "byApiKey": ttsString(entry["apiKeyMasked"]) + "|" + model + "|" + provider, "byEndpoint": ttsString(entry["endpoint"]) + "|" + model + "|" + provider} {
				if key != "" {
					day[group] = map[string]any{key: values}
				}
			}
			consume(day, ttsString(entry["timestamp"]))
		}
	}
	live := u.LiveUsage
	if live == nil {
		live = &UsageLive{History: database.Usage{DB: u.DB}}
	}
	if snapshot, err := live.Snapshot(ctx); err == nil {
		for k, v := range snapshot {
			stats[k] = v
		}
	}
	buckets := make([]map[string]any, 10)
	minute := now.Truncate(time.Minute)
	for i := range buckets {
		buckets[i] = map[string]any{"requests": float64(0), "promptTokens": float64(0), "completionTokens": float64(0), "cost": float64(0)}
	}
	recent, err := (database.Usage{DB: u.DB}).History(ctx, "", "", minute.Add(-9*time.Minute).UTC().Format(time.RFC3339), "")
	if err != nil {
		return nil, err
	}
	for _, entry := range recent {
		stamp, err := time.Parse(time.RFC3339, ttsString(entry["timestamp"]))
		if err != nil {
			continue
		}
		index := 9 - int(minute.Sub(stamp.Truncate(time.Minute))/time.Minute)
		if index < 0 || index >= 10 {
			continue
		}
		bucket := buckets[index]
		tokens, _ := entry["tokens"].(map[string]any)
		bucket["requests"] = pxpipeNum(bucket["requests"]) + 1
		bucket["promptTokens"] = pxpipeNum(bucket["promptTokens"]) + pxpipeNum(tokens["prompt_tokens"])
		bucket["completionTokens"] = pxpipeNum(bucket["completionTokens"]) + pxpipeNum(tokens["completion_tokens"])
		bucket["cost"] = pxpipeNum(bucket["cost"]) + pxpipeNum(entry["cost"])
	}
	stats["last10Minutes"] = buckets
	return stats, nil
}
func maskUsageKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 12 {
		return key[:1] + "***"
	}
	return key[:8] + "***" + key[len(key)-4:]
}

package handlers

import (
	"strings"
	"testing"
	"time"
)

func TestDeepSeekUsageNormalizesBalances(t *testing.T) {
	result := normalizeDeepSeekUsage(map[string]any{"is_available": true, "balance_infos": []any{map[string]any{"currency": "usd", "total_balance": "12.5"}, map[string]any{"currency": ""}}})
	quota := result["quotas"].(map[string]any)["Balance (USD)"].(map[string]any)
	if result["plan"] != "DeepSeek" || quota["total"] != 12.5 || quota["remainingPercentage"] != 100.0 || quota["isCreditBalance"] != true {
		t.Fatal(result)
	}
	if normalizeDeepSeekUsage(map[string]any{})["message"] != "DeepSeek connected. No balance data returned." {
		t.Fatal("empty balance")
	}
}

func TestOllamaUsageConvertsRatiosAndMonthlyReset(t *testing.T) {
	created := time.Now().UTC().AddDate(0, -1, -1).Format(time.RFC3339Nano)
	result := normalizeOllamaUsage(map[string]any{"limits": map[string]any{"session": map[string]any{"usage": 0.25}, "monthly": map[string]any{"usage": 1.4}}}, map[string]any{"Plan": "free", "CreatedAt": created})
	session := result["quotas"].(map[string]any)["Session (5h)"].(map[string]any)
	monthly := result["quotas"].(map[string]any)["Monthly"].(map[string]any)
	if result["plan"] != "Free" || session["used"] != 25.0 || session["remainingPercentage"] != 75.0 || monthly["used"] != 100.0 || !strings.Contains(ttsString(monthly["resetAt"]), "T") {
		t.Fatal(result)
	}
}

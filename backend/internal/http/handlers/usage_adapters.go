package handlers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
)

func openCodePercent(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return 0, false
		}
		return typed, true
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, false
		}
		return number, true
	default:
		return 0, false
	}
}

func (u *UsageContracts) openCodeUsage(r *http.Request, provider, token string) map[string]any {
	label := "OpenCode Zen"
	target := "https://opencode.ai/zen/v1/usage"
	if provider == "opencode-go" {
		label = "OpenCode Go"
		target = "https://opencode.ai/zen/go/v1/usage"
	}
	if strings.TrimSpace(token) == "" {
		return map[string]any{"message": label + " API key not available. Add a key to view usage."}
	}
	status, data, err := u.resetJSON(r.Context(), "GET", target, map[string]string{"Authorization": "Bearer " + strings.TrimSpace(token), "Accept": "application/json"}, nil)
	if err != nil {
		return map[string]any{"message": label + " error: " + err.Error()}
	}
	if status == 401 {
		return map[string]any{"plan": label, "message": label + " authentication failed. Check the API key."}
	}
	if status == 403 {
		failure, _ := data["error"].(map[string]any)
		message := label + " access forbidden for this API key."
		if failure["type"] == "EntitlementError" {
			requirement := "billing"
			if provider == "opencode-go" {
				requirement = "subscription"
			}
			message = label + " " + requirement + " required for this API key."
		}
		return map[string]any{"plan": label, "message": message}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"plan": label, "message": fmt.Sprintf("%s usage API error (%d).", label, status)}
	}
	usage, ok := data["usage"].(map[string]any)
	if !ok {
		return map[string]any{"plan": label, "message": label + " usage response did not contain quota data."}
	}
	quotas := map[string]any{}
	for _, period := range []struct{ key, name string }{{"rolling", "Rolling"}, {"weekly", "Weekly"}, {"monthly", "Monthly"}} {
		quota, _ := usage[period.key].(map[string]any)
		percent, valid := openCodePercent(quota["percent"])
		if !valid {
			continue
		}
		used := math.Max(0, math.Min(100, percent))
		quotas[period.name] = map[string]any{"used": used, "total": 100, "remaining": 100 - used, "remainingPercentage": 100 - used, "resetAt": usageResetDate(quota["resetsAt"]), "unlimited": false}
	}
	if len(quotas) == 0 {
		return map[string]any{"plan": label, "message": label + " usage response did not contain valid quota data."}
	}
	return map[string]any{"plan": label, "quotas": quotas}
}

func githubQuota(value any) map[string]any {
	quota, ok := value.(map[string]any)
	if !ok {
		return map[string]any{"used": 0, "total": 0, "unlimited": true}
	}
	return map[string]any{"used": usageNumber(quota["entitlement"]) - usageNumber(quota["remaining"]), "total": usageNumber(quota["entitlement"]), "remaining": usageNumber(quota["remaining"]), "unlimited": quota["unlimited"] == true}
}

func (u *UsageContracts) githubUsage(r *http.Request, token string) (map[string]any, int, error) {
	if token == "" {
		return nil, 500, fmt.Errorf("Failed to fetch GitHub usage: No GitHub access token available. Please re-authorize the connection.")
	}
	status, data, err := u.resetJSON(r.Context(), "GET", "https://api.github.com/copilot_internal/user", map[string]string{"Authorization": "token " + token, "Accept": "application/json", "X-GitHub-Api-Version": "2025-04-01", "User-Agent": "GitHubCopilotChat/0.38.0", "Editor-Version": "vscode/1.100.0", "Editor-Plugin-Version": "copilot-chat/0.26.7"}, nil)
	if err != nil {
		return nil, 500, fmt.Errorf("Failed to fetch GitHub usage: %s", err.Error())
	}
	if status < 200 || status >= 300 {
		return nil, 500, fmt.Errorf("Failed to fetch GitHub usage: GitHub API error: %s", ttsString(data))
	}
	if snapshots, ok := data["quota_snapshots"].(map[string]any); ok {
		reset := usageResetDate(data["quota_reset_date"])
		result := map[string]any{}
		for _, key := range []string{"chat", "completions", "premium_interactions"} {
			quota := githubQuota(snapshots[key])
			quota["resetAt"] = reset
			result[key] = quota
		}
		return map[string]any{"plan": data["copilot_plan"], "resetDate": data["quota_reset_date"], "quotas": result}, 200, nil
	}
	if data["monthly_quotas"] != nil || data["limited_user_quotas"] != nil {
		monthly, _ := data["monthly_quotas"].(map[string]any)
		used, _ := data["limited_user_quotas"].(map[string]any)
		reset := usageResetDate(data["limited_user_reset_date"])
		return map[string]any{"plan": firstNonEmpty(data["copilot_plan"], data["access_type_sku"]), "resetDate": data["limited_user_reset_date"], "quotas": map[string]any{"chat": map[string]any{"used": usageNumber(used["chat"]), "total": usageNumber(monthly["chat"]), "unlimited": false, "resetAt": reset}, "completions": map[string]any{"used": usageNumber(used["completions"]), "total": usageNumber(monthly["completions"]), "unlimited": false, "resetAt": reset}}}, 200, nil
	}
	return map[string]any{"message": "GitHub Copilot connected. Unable to parse quota data."}, 200, nil
}

func qoderQuota(value any) map[string]any {
	quota, _ := value.(map[string]any)
	return map[string]any{"total": usageNumber(quota["total"]), "used": usageNumber(quota["used"]), "remaining": usageNumber(quota["remaining"]), "unit": firstNonEmpty(quota["unit"], "credits")}
}

func (u *UsageContracts) qoderUsage(r *http.Request, token string) map[string]any {
	if token == "" {
		return map[string]any{"message": "Qoder usage unavailable: no access token"}
	}
	status, data, err := u.resetJSON(r.Context(), "GET", "https://openapi.qoder.sh/api/v2/quota/usage", map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json"}, nil)
	if err != nil {
		return map[string]any{"message": "Qoder connected. Unable to fetch usage: " + err.Error()}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"message": fmt.Sprintf("Qoder connected. Usage fetch returned %d.", status)}
	}
	if data == nil {
		return map[string]any{"message": "Qoder connected. Usage response was not JSON."}
	}
	expires := usageNumber(data["expiresAt"])
	var reset any
	var expiresValue any
	if expires > 0 {
		reset = usageResetDate(expires)
		expiresValue = expires
	}
	user, organization := qoderQuota(data["userQuota"]), qoderQuota(data["orgResourcePackage"])
	user["resetAt"], organization["resetAt"] = reset, reset
	return map[string]any{"quotas": map[string]any{"user": user, "organization": organization}, "totalUsagePercentage": usageNumber(data["totalUsagePercentage"]), "isQuotaExceeded": nodeTruthy(data["isQuotaExceeded"]), "expiresAt": expiresValue}
}

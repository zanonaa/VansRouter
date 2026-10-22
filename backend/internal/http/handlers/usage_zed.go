package handlers

import (
	"math"
	"net/http"
	"strings"
)

func zedPlan(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return "Zed"
	case "zed_free":
		return "Zed Free"
	case "zed_pro":
		return "Zed Pro"
	case "zed_pro_trial":
		return "Zed Pro Trial"
	case "zed_student":
		return "Zed Student"
	case "zed_business":
		return "Zed Business"
	default:
		words := strings.Fields(strings.ReplaceAll(value, "_", " "))
		for index, word := range words {
			words[index] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
		return strings.Join(words, " ")
	}
}

func zedLimit(value any) (bool, float64) {
	switch typed := value.(type) {
	case nil:
		return false, 0
	case string:
		if strings.TrimSpace(typed) == "unlimited" {
			return true, 0
		}
		number, ok := usageFinite(typed)
		if !ok {
			return false, 0
		}
		return false, math.Max(0, number)
	case map[string]any:
		if typed["unlimited"] == true {
			return true, 0
		}
		if number, ok := usageFinite(firstNonEmpty(typed["limited"], typed["Limited"])); ok {
			return false, math.Max(0, number)
		}
	}
	if number, ok := usageFinite(value); ok {
		return false, math.Max(0, number)
	}
	return false, 0
}

func zedQuota(usedRaw, limitRaw any, reset any) map[string]any {
	used := math.Max(0, usageNumber(usedRaw))
	unlimited, total := zedLimit(limitRaw)
	if unlimited {
		return map[string]any{"used": used, "total": 0, "remainingPercentage": 100, "resetAt": reset, "unlimited": true}
	}
	if total <= 0 {
		return map[string]any{"used": used, "total": 0, "remainingPercentage": 0, "resetAt": reset, "unlimited": false}
	}
	used = math.Min(used, total)
	return map[string]any{"used": used, "total": total, "remainingPercentage": (total - used) / total * 100, "resetAt": reset, "unlimited": false}
}

func parseZedUsage(data map[string]any) map[string]any {
	plan, _ := data["plan"].(map[string]any)
	planID := ttsString(firstNonEmpty(plan["plan_v3"], plan["plan_v2"], plan["plan"], data["plan_v3"]))
	period, _ := firstNonEmpty(plan["subscription_period"], plan["subscriptionPeriod"]).(map[string]any)
	reset := usageResetDate(firstNonEmpty(period["ended_at"], period["endedAt"]))
	usage, _ := plan["usage"].(map[string]any)
	quotas := map[string]any{}
	predictions, _ := firstNonEmpty(usage["edit_predictions"], usage["editPredictions"]).(map[string]any)
	if predictions != nil {
		quotas["Edit Predictions"] = zedQuota(predictions["used"], predictions["limit"], reset)
	}
	requests, _ := firstNonEmpty(usage["model_requests"], usage["modelRequests"]).(map[string]any)
	var message any
	if requests != nil {
		limit := requests["limit"]
		if limit == nil {
			limit = requests
		}
		unlimited, total := zedLimit(limit)
		if unlimited || total > 0 {
			quotas["Hosted Model Requests"] = zedQuota(requests["used"], limit, reset)
		} else {
			message = "Hosted AI models are billed per token (not request count). Edit Predictions are tracked below. Token spend is on dashboard.zed.dev."
		}
	}
	label := zedPlan(planID)
	if nodeTruthy(firstNonEmpty(plan["trial_started_at"], plan["trialStartedAt"])) && !strings.Contains(strings.ToLower(label), "trial") {
		label += " (Trial active)"
	}
	if nodeTruthy(firstNonEmpty(plan["has_overdue_invoices"], plan["hasOverdueInvoices"])) {
		message = "This Zed account has overdue invoices. Usage may be blocked until billing is resolved."
	}
	return map[string]any{"plan": label, "quotas": quotas, "message": message, "hasOverdueInvoices": nodeTruthy(firstNonEmpty(plan["has_overdue_invoices"], plan["hasOverdueInvoices"])), "trialStarted": nodeTruthy(firstNonEmpty(plan["trial_started_at"], plan["trialStartedAt"])), "planId": firstNonEmpty(planID, nil), "resetAt": reset}
}

func (u *UsageContracts) zedUsage(r *http.Request, connection map[string]any) map[string]any {
	token := strings.TrimSpace(ttsString(connection["accessToken"]))
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	user := ttsString(metadata["userId"])
	if token == "" {
		return map[string]any{"message": "Zed access token not available. Re-connect Zed to view quota."}
	}
	if user == "" {
		return map[string]any{"message": "Zed credential is missing user id. Re-connect Zed to view quota."}
	}
	headers := map[string]string{"Accept": "application/json", "Authorization": user + " " + token}
	if system := ttsString(metadata["systemId"]); system != "" {
		headers["x-zed-system-id"] = system
	}
	status, data, err := u.resetJSON(r.Context(), "GET", "https://cloud.zed.dev/client/users/me", headers, nil)
	if err != nil {
		return map[string]any{"message": "Zed error: " + err.Error()}
	}
	if status == 401 || status == 403 {
		return map[string]any{"message": "Zed authentication failed. Sign in again from the dashboard or Zed editor."}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"message": "Zed error: " + ttsString(firstNonEmpty(data["message"], data["error"], "Failed to fetch quota"))}
	}
	return parseZedUsage(data)
}

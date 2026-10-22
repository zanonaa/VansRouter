package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func parseKiroUsage(data map[string]any) map[string]any {
	items, _ := data["usageBreakdownList"].([]any)
	quotas := map[string]any{}
	reset := usageResetDate(firstNonEmpty(data["nextDateReset"], data["resetDate"]))
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		resource := strings.ToLower(ttsString(item["resourceType"]))
		if resource == "" {
			resource = "unknown"
		}
		used, total := usageNumber(item["currentUsageWithPrecision"]), usageNumber(item["usageLimitWithPrecision"])
		quotas[resource] = map[string]any{"used": used, "total": total, "remaining": total - used, "resetAt": reset, "unlimited": false}
		trial, _ := item["freeTrialInfo"].(map[string]any)
		if trial != nil {
			freeUsed, freeTotal := usageNumber(trial["currentUsageWithPrecision"]), usageNumber(trial["usageLimitWithPrecision"])
			quotas[resource+"_freetrial"] = map[string]any{"used": freeUsed, "total": freeTotal, "remaining": freeTotal - freeUsed, "resetAt": usageResetDate(firstNonEmpty(trial["freeTrialExpiry"], reset)), "unlimited": false}
		}
	}
	subscription, _ := data["subscriptionInfo"].(map[string]any)
	return map[string]any{"plan": firstNonEmpty(subscription["subscriptionTitle"], "Kiro"), "quotas": quotas}
}

func kiroProfile(method string, metadata map[string]any) string {
	if profile := strings.TrimSpace(ttsString(metadata["profileArn"])); profile != "" || method == "api_key" {
		return profile
	}
	if method == "google" || method == "github" {
		return "arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK"
	}
	return "arn:aws:codewhisperer:us-east-1:638616132270:profile/AAAACCCCXXXX"
}

func (u *UsageContracts) kiroUsage(r *http.Request, token string, connection map[string]any) map[string]any {
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	method := ttsString(metadata["authMethod"])
	if method == "" {
		method = "builder-id"
	}
	headers := map[string]string{}
	if method == "api_key" {
		headers["tokentype"] = "API_KEY"
	}
	if method == "external_idp" {
		headers["TokenType"] = "EXTERNAL_IDP"
	}
	profile := kiroProfile(method, metadata)
	base := map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json"}
	for key, value := range headers {
		base[key] = value
	}
	errors := []string{}
	authError := false
	record := func(name string, status int, raw []byte, err error) {
		if err != nil {
			errors = append(errors, name+":"+err.Error())
			return
		}
		if status == 401 || status == 403 {
			authError = true
		}
		errors = append(errors, fmt.Sprintf("%s:%d:%s", name, status, strings.TrimSpace(string(raw))))
	}
	query := url.Values{"isEmailRequired": {"true"}, "origin": {"AI_EDITOR"}, "resourceType": {"AGENTIC_REQUEST"}}
	getHeaders := map[string]string{}
	for key, value := range base {
		getHeaders[key] = value
	}
	getHeaders["x-amz-user-agent"] = "aws-sdk-js/1.0.0 KiroIDE"
	getHeaders["user-agent"] = "aws-sdk-js/1.0.0 KiroIDE"
	status, data, err := u.resetJSON(r.Context(), "GET", "https://codewhisperer.us-east-1.amazonaws.com/getUsageLimits?"+query.Encode(), getHeaders, nil)
	if err == nil && status >= 200 && status < 300 {
		return parseKiroUsage(data)
	}
	raw, _ := json.Marshal(data)
	record("codewhisperer-get", status, raw, err)
	postHeaders := map[string]string{}
	for key, value := range base {
		postHeaders[key] = value
	}
	postHeaders["Content-Type"] = "application/x-amz-json-1.0"
	postHeaders["x-amz-target"] = "AmazonCodeWhispererService.GetUsageLimits"
	payload := map[string]any{"origin": "AI_EDITOR", "resourceType": "AGENTIC_REQUEST"}
	if profile != "" {
		payload["profileArn"] = profile
	}
	status, data, err = u.resetJSON(r.Context(), "POST", "https://codewhisperer.us-east-1.amazonaws.com", postHeaders, payload)
	if err == nil && status >= 200 && status < 300 {
		return parseKiroUsage(data)
	}
	raw, _ = json.Marshal(data)
	record("codewhisperer-post", status, raw, err)
	q := url.Values{"origin": {"AI_EDITOR"}, "resourceType": {"AGENTIC_REQUEST"}}
	if profile != "" {
		q.Set("profileArn", profile)
	}
	status, data, err = u.resetJSON(r.Context(), "GET", "https://q.us-east-1.amazonaws.com/getUsageLimits?"+q.Encode(), base, nil)
	if err == nil && status >= 200 && status < 300 {
		return parseKiroUsage(data)
	}
	raw, _ = json.Marshal(data)
	record("q-get", status, raw, err)
	if authError && method == "idc" {
		return map[string]any{"message": "Kiro quota API is unavailable for the current AWS IAM Identity Center session. Chat may still work. If this persists after renewing your session, reconnect Kiro.", "quotas": map[string]any{}}
	}
	if authError && (method == "google" || method == "github") {
		return map[string]any{"message": "Kiro quota API authentication expired. Chat may still work.", "quotas": map[string]any{}}
	}
	if authError {
		return map[string]any{"message": "Kiro quota API rejected the current token. Chat may still work.", "quotas": map[string]any{}}
	}
	message := "Unable to fetch Kiro usage right now."
	if len(errors) > 0 {
		message = "Unable to fetch Kiro usage right now. (" + errors[len(errors)-1] + ")"
	}
	return map[string]any{"message": message, "quotas": map[string]any{}}
}

func freebuffLabel(model string) string {
	for _, entry := range providers.ModelInfo() {
		if entry["provider"] == "freebuff" && entry["id"] == "freebuff/"+model {
			return ttsString(entry["name"])
		}
	}
	return ""
}

func (u *UsageContracts) freebuffUsage(r *http.Request, token string) map[string]any {
	if token == "" {
		return map[string]any{"message": "Freebuff credential not available — connect a Freebuff login first."}
	}
	status, data, err := u.resetJSON(r.Context(), "GET", "https://www.codebuff.com/api/v1/freebuff/session", map[string]string{"Authorization": "Bearer " + token, "User-Agent": "codebuff-cli/0.0.138", "Accept": "application/json"}, nil)
	if err != nil {
		return map[string]any{"message": "Freebuff usage error: " + err.Error()}
	}
	if status == 401 {
		return map[string]any{"message": "Freebuff credential invalid or expired — re-login in the dashboard."}
	}
	if status == 403 {
		switch data["status"] {
		case "country_blocked":
			return map[string]any{"message": "Freebuff is not available in your region."}
		case "banned":
			return map[string]any{"message": "Your Freebuff account has been banned."}
		default:
			message := "Freebuff quota access denied (403)."
			if detail := ttsString(data["message"]); detail != "" {
				message = "Freebuff quota access denied (403): " + detail + "."
			}
			return map[string]any{"message": message}
		}
	}
	if status == 404 {
		return map[string]any{"plan": "Freebuff", "message": "Freebuff connected. No session quota to report right now."}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"message": fmt.Sprintf("Freebuff quota API error (%d).", status)}
	}
	limits, _ := data["rateLimitsByModel"].(map[string]any)
	if limits == nil {
		limits = map[string]any{}
	}
	if data["status"] == "active" && limits[ttsString(data["model"])] == nil && data["rateLimit"] != nil {
		limits[ttsString(data["model"])] = data["rateLimit"]
	}
	quotas := map[string]any{}
	for model, raw := range limits {
		limit, _ := raw.(map[string]any)
		if limit == nil {
			continue
		}
		used, usedOK := usageFinite(limit["recentCount"])
		total, totalOK := usageFinite(limit["limit"])
		if !usedOK {
			used = 0
		}
		if !totalOK {
			total = 0
		}
		quota := map[string]any{"used": used, "total": total, "resetAt": limit["resetAt"], "unlimited": false, "recurring": true}
		if label := freebuffLabel(model); label != "" {
			quota["displayName"] = label
		}
		quotas[model] = quota
	}
	plan := "Freebuff"
	if data["accessTier"] == "limited" {
		plan = "Freebuff (Limited)"
	}
	if len(quotas) == 0 {
		return map[string]any{"plan": plan, "message": "Freebuff connected. No session quota to report right now."}
	}
	return map[string]any{"plan": plan, "quotas": quotas}
}

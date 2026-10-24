package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"time"
)

var antigravityModels = map[string]bool{"gemini-3.8-flash-high": true, "gemini-3.8-flash-medium": true, "gemini-3.8-flash-low": true, "gemini-3.7-flash-high": true, "gemini-3.7-flash-medium": true, "gemini-3.7-flash-low": true, "gemini-3-flash-agent": true, "gemini-3.5-flash-low": true, "gemini-3.5-flash-extra-low": true, "gemini-pro-agent": true, "gemini-3.1-pro-low": true, "claude-sonnet-4-6": true, "claude-opus-4-6-thinking": true, "gpt-oss-120b-medium": true, "gemini-3-flash": true, "gemini-3.1-flash-image": true, "gemini-3-pro-image": true}
var antigravityIneligible = regexp.MustCompile(`(?i)VALIDATION_REQUIRED|not eligible for Gemini Code Assist`)

type weeklyUsageEntry struct {
	done    chan struct{}
	result  map[string]any
	expires time.Time
}

func cloudProject(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]any:
		return strings.TrimSpace(ttsString(typed["id"]))
	default:
		return ""
	}
}

func googlePlatform() float64 {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return 2
	case "darwin/amd64":
		return 1
	case "linux/arm64":
		return 4
	case "linux/amd64":
		return 3
	case "windows/amd64":
		return 5
	default:
		return 0
	}
}

func fractionQuota(model string, fraction float64, reset any, display any) map[string]any {
	remaining := math.Round(1000 * fraction)
	result := map[string]any{"used": math.Max(0, 1000-remaining), "total": 1000, "resetAt": usageResetDate(reset), "remainingPercentage": fraction * 100, "unlimited": false}
	if display != nil {
		result["displayName"] = firstNonEmpty(display, model)
	}
	return result
}

func (u *UsageContracts) googleJSON(ctx context.Context, target string, headers map[string]string, payload map[string]any) (int, map[string]any, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	status, data, err := u.resetJSON(ctx, "POST", target, headers, payload)
	raw, _ := json.Marshal(data)
	return status, data, raw, err
}

func (u *UsageContracts) geminiUsage(r *http.Request, token string, connection map[string]any) map[string]any {
	if token == "" {
		return map[string]any{"plan": "Free", "message": "Gemini CLI access token not available."}
	}
	metadata, _ := connection["providerSpecificData"].(map[string]any)
	project := cloudProject(firstNonEmpty(connection["projectId"], metadata["projectId"]))
	plan := "Free"
	if project == "" {
		_, info, _, err := u.googleJSON(r.Context(), "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/json"}, map[string]any{"metadata": map[string]any{"ideType": 9, "platform": googlePlatform(), "pluginType": 2}})
		if err == nil {
			project = cloudProject(info["cloudaicompanionProject"])
			tier, _ := info["currentTier"].(map[string]any)
			if name := ttsString(tier["name"]); name != "" {
				plan = name
			}
		}
	}
	if project == "" {
		return map[string]any{"plan": plan, "message": "Gemini CLI project ID not available. Reconnect Gemini CLI, or configure a Google Cloud project with Gemini Code Assist access before checking quota."}
	}
	status, data, _, err := u.googleJSON(r.Context(), "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota", map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/json"}, map[string]any{"project": project})
	if err != nil {
		return map[string]any{"message": "Gemini CLI error: " + err.Error()}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"plan": plan, "message": fmt.Sprintf("Gemini CLI quota error (%d).", status)}
	}
	quotas := map[string]any{}
	buckets, _ := data["buckets"].([]any)
	for _, raw := range buckets {
		bucket, _ := raw.(map[string]any)
		model := ttsString(bucket["modelId"])
		fraction, ok := bucket["remainingFraction"].(float64)
		if model == "" || !ok {
			continue
		}
		quotas[model] = fractionQuota(model, fraction, bucket["resetTime"], nil)
	}
	return map[string]any{"plan": plan, "quotas": quotas}
}

func (u *UsageContracts) antigravityUsage(r *http.Request, token string) map[string]any {
	headers := map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/json", "User-Agent": "antigravity/ide/2.1.1 darwin/arm64"}
	_, subscription, _, _ := u.googleJSON(r.Context(), "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist", headers, map[string]any{"metadata": map[string]any{"ideType": 9, "platform": googlePlatform(), "pluginType": 2}, "mode": 1})
	project := cloudProject(subscription["cloudaicompanionProject"])
	payload := map[string]any{}
	if project != "" {
		payload["project"] = project
	}
	modelHeaders := map[string]string{}
	for key, value := range headers {
		modelHeaders[key] = value
	}
	modelHeaders["X-Client-Name"] = "antigravity"
	modelHeaders["X-Client-Version"] = "2.1.1"
	status, data, raw, err := u.googleJSON(r.Context(), "https://daily-cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels", modelHeaders, payload)
	if err != nil {
		return map[string]any{"message": "Antigravity error: " + err.Error()}
	}
	if status == 403 {
		message := "Antigravity quota API access forbidden. Chat may still work."
		ineligible := antigravityIneligible.Match(raw)
		if ineligible {
			message = "Antigravity: Google marked this account ineligible (VALIDATION_REQUIRED). Chat will fail with 403 until the account owner completes verification."
		}
		return map[string]any{"message": message, "quotas": map[string]any{}, "ineligible": ineligible}
	}
	if status == 401 {
		return map[string]any{"message": "Antigravity quota API authentication expired. Chat may still work.", "quotas": map[string]any{}}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"message": fmt.Sprintf("Antigravity error: Antigravity API error: %d", status)}
	}
	paid, _ := subscription["paidTier"].(map[string]any)
	tier := ttsString(paid["id"])
	quotas := map[string]any{}
	if tier != "" && tier != "free-tier" {
		models, _ := data["models"].(map[string]any)
		for model, raw := range models {
			info, _ := raw.(map[string]any)
			quota, ok := info["quotaInfo"].(map[string]any)
			if !ok || info["isInternal"] == true || !antigravityModels[model] {
				continue
			}
			quotas[model] = fractionQuota(model, usageNumber(quota["remainingFraction"]), quota["resetTime"], info["displayName"])
		}
	}
	for key, value := range u.antigravityWeekly(r.Context(), token, project, modelHeaders, payload) {
		quotas[key] = value
	}
	current, _ := subscription["currentTier"].(map[string]any)
	return map[string]any{"plan": firstNonEmpty(current["name"], "Unknown"), "quotas": quotas, "subscriptionInfo": subscription}
}

func (u *UsageContracts) antigravityWeekly(ctx context.Context, token, project string, headers map[string]string, payload map[string]any) map[string]any {
	key := sha256.Sum256([]byte(token + "::" + project))
	u.claudeMu.Lock()
	if u.weeklyUsage == nil {
		u.weeklyUsage = map[[32]byte]*weeklyUsageEntry{}
	}
	current := u.weeklyUsage[key]
	if current != nil && current.done == nil && current.expires.After(time.Now()) {
		result := current.result
		u.claudeMu.Unlock()
		return cloneUsageMap(result)
	}
	if current != nil && current.done != nil {
		done := current.done
		u.claudeMu.Unlock()
		select {
		case <-ctx.Done():
			return map[string]any{}
		case <-done:
			return cloneUsageMap(current.result)
		}
	}
	entry := &weeklyUsageEntry{done: make(chan struct{})}
	u.weeklyUsage[key] = entry
	u.claudeMu.Unlock()
	status, data, _, err := u.googleJSON(ctx, "https://daily-cloudcode-pa.googleapis.com/v1internal:retrieveUserQuotaSummary", headers, payload)
	result := map[string]any{}
	if err == nil && status >= 200 && status < 300 {
		result = parseWeeklyQuota(data)
	}
	u.claudeMu.Lock()
	entry.result = cloneUsageMap(result)
	close(entry.done)
	entry.done = nil
	if len(result) > 0 {
		entry.expires = time.Now().Add(3 * time.Minute)
	} else if u.weeklyUsage[key] == entry {
		delete(u.weeklyUsage, key)
	}
	u.claudeMu.Unlock()
	return cloneUsageMap(result)
}

func parseWeeklyQuota(data map[string]any) map[string]any {
	groups, _ := data["groups"].([]any)
	if groups == nil {
		summary, _ := data["quotaSummary"].(map[string]any)
		groups, _ = summary["groups"].([]any)
	}
	result := map[string]any{}
	for _, raw := range groups {
		group, _ := raw.(map[string]any)
		name := ttsString(group["displayName"])
		family, label := "", ""
		switch {
		case regexp.MustCompile(`(?i)gemini`).MatchString(name):
			family, label = "gemini_weekly", "Gemini (Weekly)"
		case regexp.MustCompile(`(?i)claude|gpt`).MatchString(name):
			family, label = "claude_gpt_weekly", "Claude & GPT (Weekly)"
		default:
			continue
		}
		if result[family] != nil {
			continue
		}
		buckets, _ := group["buckets"].([]any)
		for _, rawBucket := range buckets {
			bucket, _ := rawBucket.(map[string]any)
			text := strings.ToLower(ttsString(bucket["bucketId"]) + " " + ttsString(bucket["displayName"]))
			fraction, ok := bucket["remainingFraction"].(float64)
			if bucket["disabled"] == true || !strings.Contains(text, "weekly") || !ok {
				continue
			}
			result[family] = fractionQuota(family, fraction, bucket["resetTime"], label)
			break
		}
	}
	return result
}

func cloneUsageMap(value map[string]any) map[string]any {
	raw, _ := json.Marshal(value)
	var result map[string]any
	json.Unmarshal(raw, &result)
	if result == nil {
		return map[string]any{}
	}
	return result
}

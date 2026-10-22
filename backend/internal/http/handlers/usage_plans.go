package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"
)

func usageFinite(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, !math.IsNaN(typed) && !math.IsInf(typed, 0)
	case string:
		number, err := strconvParse(strings.TrimSpace(typed))
		return number, err == nil && !math.IsNaN(number) && !math.IsInf(number, 0)
	default:
		return 0, false
	}
}

func strconvParse(value string) (float64, error) {
	if value == "" {
		return 0, fmt.Errorf("empty")
	}
	return json.Number(value).Float64()
}

func usageField(model map[string]any, snake, camel string) any {
	if value, ok := model[snake]; ok {
		return value
	}
	return model[camel]
}

func parseGLMUsage(data map[string]any) map[string]any {
	payload, _ := data["data"].(map[string]any)
	limits, _ := payload["limits"].([]any)
	quotas := map[string]any{}
	for _, raw := range limits {
		limit, _ := raw.(map[string]any)
		if limit["type"] != "TOKENS_LIMIT" && limit["type"] != "CREDIT_LIMIT" {
			continue
		}
		used := usageNumber(limit["percentage"])
		remaining := math.Max(0, 100-used)
		key := "session"
		switch {
		case usageNumber(limit["unit"]) == 3:
			key = fmt.Sprintf("Session (%vh)", limit["number"])
		case usageNumber(limit["unit"]) == 6:
			key = "Weekly (7d)"
		case limit["type"] == "TOKENS_LIMIT":
			key = "Tokens"
		default:
			key = fmt.Sprintf("Limit (%v)", limit["number"])
		}
		reset := usageNumber(limit["nextResetTime"])
		var resetAt any
		if reset > 0 {
			resetAt = time.UnixMilli(int64(reset)).UTC().Format(time.RFC3339Nano)
		}
		quotas[key] = map[string]any{"used": used, "total": 100, "remaining": remaining, "remainingPercentage": remaining, "resetAt": resetAt, "unlimited": false}
	}
	level := ttsString(payload["level"])
	plan := "Unknown"
	if level != "" {
		plan = strings.ToUpper(level[:1]) + strings.ToLower(level[1:])
	}
	return map[string]any{"plan": plan, "quotas": quotas}
}

func (u *UsageContracts) glmUsage(r *http.Request, provider, token string) map[string]any {
	if token == "" {
		return map[string]any{"message": "GLM API key not available."}
	}
	target := "https://api.z.ai/api/monitor/usage/quota/limit"
	if provider == "glm-cn" {
		target = "https://open.bigmodel.cn/api/monitor/usage/quota/limit"
	}
	status, data, err := u.resetJSON(r.Context(), "GET", target, map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json"}, nil)
	if err != nil {
		return map[string]any{"message": "GLM error: " + err.Error()}
	}
	if status == 401 {
		return map[string]any{"message": "GLM API key invalid or expired."}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"message": fmt.Sprintf("GLM quota API error (%d).", status)}
	}
	return parseGLMUsage(data)
}

func miniMaxName(model map[string]any) string {
	name := strings.TrimSpace(ttsString(usageField(model, "model_name", "modelName")))
	if name == "" {
		return "MiniMax"
	}
	if name == "MiniMax-M*" || name == "general" {
		return "M-series"
	}
	name = regexp.MustCompile(`[_-]+`).ReplaceAllString(name, " ")
	name = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(name), " ")
	words := strings.Fields(name)
	for index, word := range words {
		words[index] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
	}
	name = strings.Join(words, " ")
	name = strings.ReplaceAll(name, " To ", " to ")
	name = strings.ReplaceAll(name, " Tts ", " TTS ")
	return strings.ReplaceAll(name, " Hd ", " HD ")
}

func miniMaxPercent(model map[string]any, snake, camel string) (float64, bool) {
	value := usageField(model, snake, camel)
	if value == nil {
		return 0, false
	}
	number, ok := usageFinite(value)
	if !ok {
		return 0, false
	}
	return math.Max(0, math.Min(100, number)), true
}

func addMiniMaxQuota(quotas map[string]any, key string, model map[string]any, totalKey, totalCamel, countKey, countCamel, percentKey, percentCamel string, remains, remainsCamel string, captured time.Time, remainingCount bool) {
	total := math.Max(0, usageNumber(usageField(model, totalKey, totalCamel)))
	percent, hasPercent := miniMaxPercent(model, percentKey, percentCamel)
	if total <= 0 && !hasPercent {
		return
	}
	count := math.Max(0, usageNumber(usageField(model, countKey, countCamel)))
	if total <= 0 {
		total = 100
		if remainingCount {
			count = math.Round(total * percent / 100)
		} else {
			count = math.Round(total * (1 - percent/100))
		}
	}
	used := count
	if remainingCount {
		used = math.Max(total-count, 0)
	}
	used = math.Min(math.Max(0, used), total)
	remaining := math.Max(total-used, 0)
	percentage := remaining / total * 100
	if hasPercent {
		percentage = percent
	}
	reset := captured
	if delay := usageNumber(usageField(model, remains, remainsCamel)); delay > 0 {
		reset = captured.Add(time.Duration(delay) * time.Millisecond)
	}
	quotas[key] = map[string]any{"used": used, "total": total, "remaining": remaining, "remainingPercentage": percentage, "resetAt": reset.UTC().Format(time.RFC3339Nano), "unlimited": false}
}

func (u *UsageContracts) miniMaxUsage(r *http.Request, provider, token string) map[string]any {
	if token == "" {
		return map[string]any{"message": "MiniMax API key not available."}
	}
	urls := []string{"https://www.minimax.io/v1/token_plan/remains", "https://api.minimax.io/v1/api/openplatform/coding_plan/remains"}
	if provider == "minimax-cn" {
		urls = []string{"https://www.minimaxi.com/v1/api/openplatform/coding_plan/remains", "https://api.minimaxi.com/v1/api/openplatform/coding_plan/remains"}
	}
	last := ""
	for index, target := range urls {
		status, data, err := u.resetJSON(r.Context(), "GET", target, map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json", "Content-Type": "application/json"}, nil)
		fallback := index < len(urls)-1
		if err != nil {
			last = err.Error()
			if fallback {
				continue
			}
			break
		}
		base, _ := firstNonEmpty(data["base_resp"], data["baseResp"]).(map[string]any)
		code := usageNumber(firstNonEmpty(base["status_code"], base["statusCode"]))
		message := strings.TrimSpace(ttsString(firstNonEmpty(base["status_msg"], base["statusMsg"])))
		raw, _ := json.Marshal(data)
		if status == 401 || status == 403 || code == 1004 || regexp.MustCompile(`(?i)token plan|coding plan|invalid api key|invalid key|unauthorized|inactive`).MatchString(message+" "+string(raw)) {
			return map[string]any{"message": "MiniMax API key invalid or inactive. Use an active Token/Coding Plan key."}
		}
		if status < 200 || status >= 300 {
			last = fmt.Sprintf("MiniMax usage endpoint error (%d)", status)
			if (status == 404 || status == 405 || status >= 500) && fallback {
				continue
			}
			return map[string]any{"message": "MiniMax connected. " + last}
		}
		if code != 0 {
			detail := message
			if detail == "" {
				detail = "Upstream quota API error"
			}
			return map[string]any{"message": "MiniMax connected. " + detail}
		}
		models, _ := firstNonEmpty(data["model_remains"], data["modelRemains"]).([]any)
		quotas := map[string]any{}
		captured := time.Now()
		remainingCount := strings.Contains(target, "/coding_plan/remains")
		for _, rawModel := range models {
			model, _ := rawModel.(map[string]any)
			name := miniMaxName(model)
			addMiniMaxQuota(quotas, name+" (5h)", model, "current_interval_total_count", "currentIntervalTotalCount", "current_interval_usage_count", "currentIntervalUsageCount", "current_interval_remaining_percent", "currentIntervalRemainingPercent", "remains_time", "remainsTime", captured, remainingCount)
			addMiniMaxQuota(quotas, name+" (7d)", model, "current_weekly_total_count", "currentWeeklyTotalCount", "current_weekly_usage_count", "currentWeeklyUsageCount", "current_weekly_remaining_percent", "currentWeeklyRemainingPercent", "weekly_remains_time", "weeklyRemainsTime", captured, remainingCount)
		}
		if len(quotas) == 0 {
			return map[string]any{"message": "MiniMax connected. No quota data was returned."}
		}
		return map[string]any{"quotas": quotas}
	}
	if last == "" {
		return map[string]any{"message": "MiniMax connected. Unable to fetch usage."}
	}
	return map[string]any{"message": "MiniMax connected. Unable to fetch usage: " + last}
}

func kimiPlan(level string) string {
	names := map[string]string{"LEVEL_BASIC": "Moderato", "LEVEL_INTERMEDIATE": "Allegretto", "LEVEL_ADVANCED": "Allegro", "LEVEL_STANDARD": "Vivace"}
	if names[level] != "" {
		return names[level]
	}
	return strings.ToLower(strings.TrimPrefix(level, "LEVEL_"))
}

func (u *UsageContracts) kimiUsage(r *http.Request, connection map[string]any) map[string]any {
	apiKey, token := ttsString(connection["apiKey"]), ttsString(connection["accessToken"])
	headers := map[string]string{"Content-Type": "application/json", "Accept": "application/json"}
	switch {
	case apiKey != "":
		headers["x-api-key"] = apiKey
	case token != "":
		headers["Authorization"] = "Bearer " + token
		metadata, _ := connection["providerSpecificData"].(map[string]any)
		device := strings.TrimSpace(ttsString(metadata["deviceId"]))
		if device == "" {
			device = fmt.Sprintf("kimi-%d", time.Now().UnixMilli())
		}
		model := "Linux " + runtime.GOARCH
		if runtime.GOOS == "darwin" {
			model = "macOS " + runtime.GOARCH
		} else if runtime.GOOS == "windows" {
			model = "Windows " + runtime.GOARCH
		}
		name, _ := os.Hostname()
		if name == "" {
			name = "unknown"
		}
		headers["X-Msh-Platform"] = "9router"
		headers["X-Msh-Version"] = "0.0.0"
		headers["X-Msh-Device-Name"] = name
		headers["X-Msh-Device-Model"] = model
		headers["X-Msh-Device-Id"] = device
	default:
		return map[string]any{"message": "Kimi access token or API key not available."}
	}
	status, data, err := u.resetJSON(r.Context(), "GET", "https://api.kimi.com/coding/v1/usages", headers, nil)
	if err != nil {
		return map[string]any{"message": "Kimi Coding connected. Unable to fetch usage: " + err.Error()}
	}
	raw, _ := json.Marshal(data)
	if status < 200 || status >= 300 {
		return map[string]any{"plan": "Kimi Coding", "message": kimiUsageError(status, string(raw), data)}
	}
	usage, _ := data["usage"].(map[string]any)
	quotas := map[string]any{}
	limit, used := usageNumber(firstNonEmpty(usage["limit"], usage["Limit"])), usageNumber(firstNonEmpty(usage["used"], usage["Used"]))
	if limit > 0 {
		remaining, ok := usageFinite(firstNonEmpty(usage["remaining"], usage["Remaining"]))
		percentage := math.Max(0, limit-used) / limit * 100
		if ok {
			percentage = math.Max(0, remaining) / limit * 100
		}
		quotas["Weekly"] = map[string]any{"used": math.Max(0, used), "total": limit, "remainingPercentage": percentage, "resetAt": usageResetDate(firstNonEmpty(usage["resetTime"], usage["ResetTime"], usage["reset_at"], usage["resetAt"])), "unlimited": false}
	}
	user, _ := data["user"].(map[string]any)
	membership, _ := user["membership"].(map[string]any)
	plan := kimiPlan(ttsString(membership["level"]))
	if plan == "" {
		plan = "Kimi Coding"
	}
	if len(quotas) == 0 {
		return map[string]any{"plan": plan, "message": "Kimi Coding connected. Usage tracked per request."}
	}
	return map[string]any{"plan": plan, "quotas": quotas}
}

func kimiUsageError(status int, raw string, data map[string]any) string {
	if status == 401 {
		return "Kimi authentication expired. Please re-authorize."
	}
	details, _ := data["details"].([]any)
	detail := map[string]any{}
	if len(details) > 0 {
		detail, _ = details[0].(map[string]any)
	}
	debug, _ := firstNonEmpty(detail["debug"], data["debug"]).(map[string]any)
	localized, _ := firstNonEmpty(debug["localizedMessage"], detail["localizedMessage"]).(map[string]any)
	reason := ttsString(firstNonEmpty(debug["reason"], data["reason"]))
	message := ttsString(firstNonEmpty(localized["message"], data["message"]))
	if status == 403 && (reason == "REASON_FEATURE_NO_PERMISSION" || regexp.MustCompile(`(?i)permission_denied|do not have permission|subscribe`).MatchString(ttsString(data["code"])+" "+message+" "+raw)) {
		if message != "" {
			return message
		}
		return "Kimi connected, but this account has no permission to view usage. Subscribe to Kimi Code to access quota."
	}
	if message != "" {
		return fmt.Sprintf("Kimi Coding connected. API Error %d: %s", status, message)
	}
	return fmt.Sprintf("Kimi Coding connected. API Error %d", status)
}

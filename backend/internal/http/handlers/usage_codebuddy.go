package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"time"
)

func codeBuddyNumber(precise, plain any) float64 {
	if number, ok := usageFinite(precise); ok {
		return number
	}
	if number, ok := usageFinite(plain); ok {
		return number
	}
	return 0
}

func codeBuddyTime(value any) (time.Time, bool) {
	text := ttsString(usageResetDate(value))
	parsed, err := time.Parse(time.RFC3339Nano, text)
	return parsed, err == nil
}

func parseCodeBuddyUsage(data map[string]any, provider string) map[string]any {
	label := "CodeBuddy CN"
	if provider == "codebuddy-intl" {
		label = "CodeBuddy Intl"
	}
	if usageNumber(data["code"]) != 0 {
		return map[string]any{"message": label + " quota error: " + firstNonEmptyString(data["msg"], "unknown")}
	}
	payload, _ := data["data"].(map[string]any)
	response, _ := payload["Response"].(map[string]any)
	inner, _ := response["Data"].(map[string]any)
	accounts, ok := inner["Accounts"].([]any)
	if !ok {
		return map[string]any{"message": label + " connected. Usage payload malformed."}
	}
	if len(accounts) == 0 {
		return map[string]any{"message": label + " connected. No credit package found."}
	}
	type pack struct {
		data map[string]any
		end  time.Time
	}
	refills, bonuses := []pack{}, []pack{}
	for _, raw := range accounts {
		account, _ := raw.(map[string]any)
		cycle, cycleOK := codeBuddyTime(account["CycleEndTime"])
		deduction, deductionOK := codeBuddyTime(account["DeductionEndTime"])
		item := pack{data: account, end: cycle}
		if cycleOK && deductionOK && deduction.Sub(cycle) > 48*time.Hour {
			refills = append(refills, item)
		} else {
			bonuses = append(bonuses, item)
		}
	}
	sort.SliceStable(refills, func(i, j int) bool { return refills[i].end.Before(refills[j].end) })
	sort.SliceStable(bonuses, func(i, j int) bool { return bonuses[i].end.Before(bonuses[j].end) })
	quotas := map[string]any{}
	counts := map[string]int{}
	for _, item := range refills {
		cadence := "Monthly"
		if start, ok := codeBuddyTime(item.data["CycleStartTime"]); ok && !item.end.IsZero() {
			days := item.end.Sub(start).Hours() / 24
			if days <= 1.5 {
				cadence = "Daily"
			} else if days <= 10 {
				cadence = "Weekly"
			}
		}
		counts[cadence]++
		name := cadence
		if counts[cadence] > 1 {
			name = fmt.Sprintf("%s %d", cadence, counts[cadence])
		}
		quotas[name] = map[string]any{"used": codeBuddyNumber(item.data["CycleCapacityUsedPrecise"], item.data["CycleCapacityUsed"]), "total": codeBuddyNumber(item.data["CycleCapacitySizePrecise"], item.data["CycleCapacitySize"]), "resetAt": usageResetDate(item.data["CycleEndTime"]), "unlimited": false, "recurring": true}
	}
	for index, item := range bonuses {
		quotas[fmt.Sprintf("Bonus Pack %d", index+1)] = map[string]any{"used": codeBuddyNumber(item.data["CapacityUsedPrecise"], item.data["CapacityUsed"]), "total": codeBuddyNumber(item.data["CapacitySizePrecise"], item.data["CapacitySize"]), "resetAt": usageResetDate(item.data["CycleEndTime"]), "unlimited": false, "recurring": false}
	}
	base := map[string]any{}
	if len(refills) > 0 {
		base = refills[0].data
	} else if len(accounts) > 0 {
		base, _ = accounts[0].(map[string]any)
	}
	return map[string]any{"plan": firstNonEmpty(base["PackageName"], base["SubProductName"], "CodeBuddy"), "quotas": quotas}
}

func (u *UsageContracts) codeBuddyUsage(r *http.Request, provider string, connection map[string]any) map[string]any {
	label := "CodeBuddy CN"
	target := "https://copilot.tencent.com/v2/billing/meter/get-user-resource"
	headers := map[string]string{"User-Agent": "CLI/2.108.1 CodeBuddy/2.108.1", "X-Product": "SaaS", "X-IDE-Type": "CLI", "X-IDE-Name": "CLI"}
	if provider == "codebuddy-intl" {
		label = "CodeBuddy Intl"
		target = "https://www.codebuddy.ai/v2/billing/meter/get-user-resource"
		headers = map[string]string{"User-Agent": "IDE/2.108.1 CodeBuddy/2.108.1", "X-Product": "SaaS", "X-IDE-Type": "IDE", "X-IDE-Name": "IDE"}
	}
	headers["x-requested-with"] = "XMLHttpRequest"
	headers["x-codebuddy-request"] = "1"
	headers["Content-Type"] = "application/json"
	headers["Accept"] = "application/json"
	token := ttsString(firstNonEmpty(connection["accessToken"], connection["apiKey"]))
	if token == "" {
		return map[string]any{"message": label + " credential not available."}
	}
	headers["Authorization"] = "Bearer " + token
	status, data, err := u.resetJSON(r.Context(), "POST", target, headers, map[string]any{})
	if err != nil {
		return map[string]any{"message": label + " error: " + err.Error()}
	}
	if status == 401 || status == 403 {
		return map[string]any{"message": label + " credential invalid or expired."}
	}
	if status < 200 || status >= 300 {
		return map[string]any{"message": fmt.Sprintf("%s quota API error (%d).", label, status)}
	}
	return parseCodeBuddyUsage(data, provider)
}

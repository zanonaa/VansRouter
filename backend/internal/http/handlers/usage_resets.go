package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
)

func (u *UsageContracts) resetJSON(ctx context.Context, method, target string, headers map[string]string, payload any) (int, map[string]any, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return 0, nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	client := u.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, nil, err
	}
	defer response.Body.Close()
	var data map[string]any
	// These services accept non-JSON responses as null, matching res.json().catch.
	_ = json.NewDecoder(response.Body).Decode(&data)
	return response.StatusCode, data, nil
}

var claudeResetGrantPattern = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)

func (u *UsageContracts) claudeReset(w http.ResponseWriter, r *http.Request) {
	connection, ok := u.connection(w, r)
	if !ok {
		return
	}
	if connection["provider"] != "claude" || connection["authType"] != "oauth" {
		nodeReply(w, 400, map[string]any{"error": "Limit reset is only available for Claude OAuth connections."})
		return
	}
	body, _ := poolInput(r)
	grant := ttsString(body["grantId"])
	token := ttsString(connection["accessToken"])
	fail := func(err error) { nodeReply(w, 500, map[string]any{"error": err.Error()}) }
	if token == "" {
		fail(fmt.Errorf("No Claude access token available. Please re-authorize the connection."))
		return
	}
	if !claudeResetGrantPattern.MatchString(grant) {
		fail(fmt.Errorf("Invalid reset grant id."))
		return
	}
	headers := map[string]string{"Authorization": "Bearer " + token, "anthropic-beta": "oauth-2025-04-20", "anthropic-version": "2023-06-01", "Content-Type": "application/json"}
	status, profile, err := u.resetJSON(r.Context(), "GET", "https://api.anthropic.com/api/oauth/profile", headers, nil)
	if err != nil {
		fail(err)
		return
	}
	organization, _ := profile["organization"].(map[string]any)
	org := ttsString(organization["uuid"])
	if status < 200 || status >= 300 || org == "" {
		fail(fmt.Errorf("Cannot resolve Claude organization (%d).", status))
		return
	}
	status, data, err := u.resetJSON(r.Context(), "POST", "https://api.anthropic.com/api/organizations/"+url.PathEscape(org)+"/reset_rate_limits", headers, map[string]any{"program": "cedar_ember", "grant_id": grant, "request_id": newUUIDFallback()})
	if err != nil {
		fail(err)
		return
	}
	upstreamError, _ := data["error"].(map[string]any)
	result := map[string]any{"ok": status >= 200 && status < 300 && data["result"] == "reset", "status": status, "result": firstNonEmpty(data["result"]), "reason": firstNonEmpty(data["reason"]), "resetsLeft": data["resets_left"], "message": firstNonEmpty(upstreamError["message"])}
	if result["ok"] == true {
		nodeReply(w, 200, result)
		return
	}
	responseStatus := 409
	if status >= 400 && status < 500 {
		responseStatus = status
	}
	if !nodeTruthy(result["message"]) {
		reason := ttsString(firstNonEmpty(result["reason"], result["result"], "unknown"))
		result["message"] = "Reset not applied: " + reason
	}
	nodeReply(w, responseStatus, result)
}

func codexCreditDate(value any) any {
	if value == nil {
		return nil
	}
	if number, ok := value.(float64); ok {
		if number <= 0 {
			return nil
		}
		if number < 1e12 {
			number *= 1000
		}
		return time.UnixMilli(int64(number)).UTC().Format("2006-01-02T15:04:05.000Z")
	}
	if text, ok := value.(string); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, text); err == nil {
			return parsed.UTC().Format("2006-01-02T15:04:05.000Z")
		}
	}
	return nil
}

func (u *UsageContracts) codexCredits(w http.ResponseWriter, r *http.Request) {
	connection, ok := u.connection(w, r)
	if !ok {
		return
	}
	if connection["provider"] != "codex" {
		nodeReply(w, 400, map[string]any{"error": "Codex reset credits are only available for Codex connections."})
		return
	}
	if connection["authType"] != "oauth" && connection["authType"] != "access_token" {
		nodeReply(w, 400, map[string]any{"error": "Codex reset credits require an OAuth or access-token connection."})
		return
	}
	token := ttsString(connection["accessToken"])
	fail := func(err error) { nodeReply(w, 500, map[string]any{"error": err.Error()}) }
	if token == "" {
		fail(fmt.Errorf("No Codex access token available. Please re-authorize the connection."))
		return
	}
	headers := map[string]string{"Authorization": "Bearer " + token, "Accept": "application/json"}
	endpoint := "https://chatgpt.com/backend-api/wham/rate-limit-reset-credits"
	var payload any
	redeemID := ""
	if r.Method == "POST" {
		endpoint += "/consume"
		redeemID = newUUIDFallback()
		payload = map[string]any{"redeem_request_id": redeemID}
		headers["Content-Type"] = "application/json"
	} else {
		headers["OpenAI-Beta"] = "codex-1"
		headers["originator"] = "codex_cli_rs"
		metadata, _ := connection["providerSpecificData"].(map[string]any)
		if account := ttsString(metadata["chatgptAccountId"]); account != "" {
			headers["ChatGPT-Account-ID"] = account
		}
	}
	status, data, err := u.resetJSON(r.Context(), r.Method, endpoint, headers, payload)
	if err != nil {
		fail(err)
		return
	}
	if r.Method == "GET" {
		if status < 200 || status >= 300 {
			message := ttsString(firstNonEmpty(data["message"], data["error"], data["detail"]))
			if message == "" {
				message = fmt.Sprintf("Codex reset credits API unavailable (%d).", status)
			}
			fail(fmt.Errorf("%s", message))
			return
		}
		credits := []map[string]any{}
		entries, _ := data["credits"].([]any)
		for _, raw := range entries {
			entry, _ := raw.(map[string]any)
			credits = append(credits, map[string]any{"status": firstNonEmpty(entry["status"], "unknown"), "grantedAt": codexCreditDate(firstNonEmpty(entry["granted_at"], entry["grantedAt"])), "expiresAt": codexCreditDate(firstNonEmpty(entry["expires_at"], entry["expiresAt"]))})
		}
		count := pxpipeNum(firstNonEmpty(data["available_count"], data["availableCount"]))
		if count < 0 {
			count = 0
		}
		nodeReply(w, 200, map[string]any{"availableCount": count, "credits": credits})
		return
	}
	windows := pxpipeNum(data["windows_reset"])
	code := data["code"]
	if status >= 200 && status < 300 && (code == "reset" || windows > 0) {
		nodeReply(w, 200, map[string]any{"code": code, "reset": true, "windows_reset": windows, "redeemRequestId": redeemID, "credit": data["credit"]})
		return
	}
	if status >= 200 && status < 300 && code == "no_credit" {
		nodeReply(w, 409, map[string]any{"code": "no_credit", "reset": false, "windows_reset": windows, "message": "No Codex reset credits available."})
		return
	}
	responseStatus := 502
	if status >= 400 && status < 500 {
		responseStatus = status
	}
	nodeReply(w, responseStatus, map[string]any{"code": firstNonEmpty(code, "unknown_response"), "reset": false, "windows_reset": windows, "message": firstNonEmpty(data["message"], "Codex reset credit consume returned an unexpected response.")})
}

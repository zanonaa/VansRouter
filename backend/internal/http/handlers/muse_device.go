package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func museHeaders() map[string]string {
	return map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json", "x-api-version": "1.0.0"}
}
func (h *OAuthEngine) museDevice(w http.ResponseWriter, r *http.Request) {
	c := h.config("muse")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: muse"})
		return
	}
	status, data, raw, err := h.deviceJSON(r.Context(), "POST", c.DeviceCodeURL, museHeaders(), strings.NewReader(url.Values{"client_id": {c.ClientID}}.Encode()))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Muse Code device code request failed"})
		return
	}
	if status < 200 || status >= 300 {
		nodeReply(w, 500, map[string]string{"error": "Muse Code device code request failed: " + string(raw)})
		return
	}
	if data == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid Muse device response"})
		return
	}
	verifier, _, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	data["codeVerifier"] = verifier
	nodeReply(w, 200, data)
}
func (h *OAuthEngine) museWait(ctx context.Context, duration time.Duration) error {
	if h.RetryWait != nil {
		return h.RetryWait(ctx, duration)
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (h *OAuthEngine) museMint(ctx context.Context, access string) (map[string]any, error) {
	target := "https://api.meta.ai/muse-code/key"
	if c := h.config("muse"); c != nil && c.MuseKeyURL != "" {
		target = c.MuseKeyURL
	}
	// A origem usa redirect:error: não seguir redirects nem encaminhar credencial.
	client := *h.client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	var status int
	var raw []byte
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", target, strings.NewReader(`{"onboard":true}`))
		if err != nil {
			return nil, err
		}
		for key, value := range map[string]string{"Accept": "application/json", "Authorization": "Bearer " + access, "Content-Type": "application/json", "x-api-version": "1.0.0"} {
			req.Header.Set(key, value)
		}
		response, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		status = response.StatusCode
		raw, err = io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		if (status != 429 && status < 500) || attempt == 2 {
			break
		}
		if err = h.museWait(ctx, time.Duration(attempt+1)*5*time.Second); err != nil {
			return nil, err
		}
	}
	var payload map[string]any
	parseErr := json.Unmarshal(raw, &payload)
	if status < 200 || status >= 300 {
		message := fmt.Sprintf("%d %s", status, string(raw[:min(len(raw), 200)]))
		if parseErr == nil && (nodeTruthy(payload["title"]) || nodeTruthy(payload["detail"])) {
			message = strings.Join(nonEmptyStrings(payload["title"], payload["detail"]), ": ")
			if nodeTruthy(payload["action_url"]) {
				message += " — " + jsString(payload["action_url"])
			}
		}
		return nil, failText("Muse Code key mint failed: " + message)
	}
	if parseErr != nil || payload == nil {
		return nil, failText("Muse Code key mint returned invalid JSON")
	}
	if payload["is_subs_active"] == false {
		return nil, failText("Muse Code subscription is inactive — activate it on muse.ai first")
	}
	action := firstNonEmpty(payload["action_url"], payload["require_payment_action_url"])
	if !nodeTruthy(payload["api_key"]) && (nodeTruthy(payload["require_payment"]) || nodeTruthy(action)) {
		message := "Muse Code subscription required"
		if nodeTruthy(action) {
			message += ": " + jsString(action)
		}
		return nil, failText(message)
	}
	if !nodeTruthy(payload["api_key"]) {
		return nil, failText("Muse Code key response is missing api_key")
	}
	return payload, nil
}
func (h *OAuthEngine) musePoll(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid or empty request body"})
		return
	}
	code, _ := body["deviceCode"].(string)
	if code == "" {
		nodeReply(w, 400, map[string]string{"error": "Missing device code"})
		return
	}
	if !nodeTruthy(body["codeVerifier"]) {
		nodeReply(w, 400, map[string]string{"error": "Missing code verifier"})
		return
	}
	c := h.config("muse")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: muse"})
		return
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "device_code": {code}, "client_id": {c.ClientID}}
	status, data, raw, err := h.deviceJSON(r.Context(), "POST", c.TokenURL, museHeaders(), strings.NewReader(form.Encode()))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Muse token poll failed"})
		return
	}
	if data == nil {
		data = map[string]any{"error": "invalid_response", "error_description": string(raw)}
	}
	errorCode, _ := data["error"].(string)
	ok := (status >= 200 && status < 300) || errorCode == "authorization_pending" || errorCode == "slow_down"
	access, _ := data["access_token"].(string)
	if !ok || access == "" {
		result := map[string]any{"success": false}
		for source, target := range map[string]string{"error": "error", "error_description": "errorDescription"} {
			if value, exists := data[source]; exists {
				result[target] = value
			}
		}
		if ok {
			if errorCode == "authorization_pending" || errorCode == "slow_down" {
				result["pending"] = errorCode == "authorization_pending"
				if !nodeTruthy(result["errorDescription"]) && nodeTruthy(data["message"]) {
					result["errorDescription"] = data["message"]
				}
			} else {
				if !nodeTruthy(result["error"]) {
					result["error"] = "no_access_token"
				}
				if !nodeTruthy(result["errorDescription"]) {
					result["errorDescription"] = legacyMessage(data)
				}
			}
		}
		nodeReply(w, 200, result)
		return
	}
	key, err := h.museMint(r.Context(), access)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	var tier any
	if nodeTruthy(key["subs_tier_name"]) {
		tier = key["subs_tier_name"]
	}
	mapped := map[string]any{"provider": "muse", "authType": "oauth", "accessToken": key["api_key"], "refreshToken": nil, "expiresIn": nil, "expiresAt": nil, "testStatus": "active", "providerSpecificData": map[string]any{"authMethod": "device_code", "oauthAccessToken": access, "subscriptionTier": tier}}
	if email := strings.ToLower(firstNonEmptyString(key["user_email"])); email != "" {
		mapped["email"] = email
	}
	saved, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"]}})
}

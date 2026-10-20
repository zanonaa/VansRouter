package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func glmSuccess(value any) bool {
	switch value {
	case nil, float64(0), float64(200), "0", "200":
		return true
	}
	return false
}
func (h *OAuthEngine) glmBusiness(ctx context.Context, method, target string, headers map[string]string, body any, label string) (any, error) {
	var payload string
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = string(raw)
	}
	status, data, raw, err := h.deviceJSON(ctx, method, target, headers, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		if len(raw) > 200 {
			raw = raw[:200]
		}
		return nil, fmt.Errorf("Z.ai %s request failed (HTTP %d): %s", label, status, string(raw))
	}
	if data == nil {
		return nil, fmt.Errorf("Z.ai %s response is not valid JSON", label)
	}
	if !glmSuccess(data["code"]) || data["success"] == false {
		if nodeTruthy(data["msg"]) {
			return nil, failText(jsString(data["msg"]))
		}
		return nil, fmt.Errorf("Z.ai %s returned business error %v", label, data["code"])
	}
	if value, ok := data["data"]; ok && value != nil {
		return value, nil
	}
	return data, nil
}
func defaultGLMName(value any) bool {
	s := strings.ToLower(strings.TrimSpace(jsString(value)))
	return s == "default" || strings.Contains(s, "默认机构") || strings.Contains(s, "默认项目")
}
func glmLocation(info any) (string, string) {
	root, _ := info.(map[string]any)
	orgs, _ := root["organizations"].([]any)
	type candidate struct {
		org      map[string]any
		projects []map[string]any
	}
	candidates := []candidate{}
	for _, raw := range orgs {
		org, _ := raw.(map[string]any)
		projects, _ := org["projects"].([]any)
		eligible := []map[string]any{}
		for _, raw := range projects {
			project, _ := raw.(map[string]any)
			kind := project["projectType"]
			if kind != nil && strings.TrimSpace(jsString(kind)) == "2" {
				continue
			}
			eligible = append(eligible, project)
		}
		if nodeTruthy(org["organizationId"]) && len(eligible) > 0 {
			candidates = append(candidates, candidate{org, eligible})
		}
	}
	if len(candidates) == 0 {
		return "", ""
	}
	selected := candidates[0]
	for _, c := range candidates {
		if defaultGLMName(c.org["organizationName"]) {
			selected = c
			break
		}
	}
	project := selected.projects[0]
	for _, p := range selected.projects {
		if defaultGLMName(p["projectName"]) {
			project = p
			break
		}
	}
	if !nodeTruthy(project["projectId"]) {
		return "", ""
	}
	return jsString(selected.org["organizationId"]), jsString(project["projectId"])
}
func (h *OAuthEngine) glmPlanKey(ctx context.Context, c *providers.OAuthConfig, access string) (string, string, error) {
	login, err := h.glmBusiness(ctx, "POST", c.BusinessLoginURL, map[string]string{"Content-Type": "application/json"}, map[string]string{"token": access}, "Z.ai business login")
	if err != nil {
		return "", "", err
	}
	credentials, _ := login.(map[string]any)
	token := firstNonEmptyString(credentials["access_token"], credentials["accessToken"])
	if token == "" {
		return "", "", failText("Z.ai business login response is missing access_token")
	}
	headers := map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/json"}
	customer, err := h.glmBusiness(ctx, "GET", c.APIBaseURL+"/api/biz/customer/getCustomerInfo", headers, nil, "customer info")
	if err != nil {
		return "", "", err
	}
	org, project := glmLocation(customer)
	if org == "" || project == "" {
		return "", "", failText("Unable to resolve Z.ai organization and project for the coding plan")
	}
	listURL := c.APIBaseURL + "/api/biz/v1/organization/" + url.PathEscape(org) + "/projects/" + url.PathEscape(project) + "/api_keys"
	keys, err := h.glmBusiness(ctx, "GET", listURL, headers, nil, "api keys")
	if err != nil {
		return "", "", err
	}
	var entry map[string]any
	if list, ok := keys.([]any); ok {
		for _, raw := range list {
			candidate, _ := raw.(map[string]any)
			if candidate["name"] == c.PlanAPIKeyName {
				entry = candidate
				break
			}
		}
	}
	if entry == nil {
		created, err := h.glmBusiness(ctx, "POST", listURL, headers, map[string]string{"name": c.PlanAPIKeyName}, "api key create")
		if err != nil {
			return "", "", err
		}
		entry, _ = created.(map[string]any)
	}
	key := firstNonEmptyString(entry["apiKey"])
	if key == "" {
		return "", "", failText("Z.ai api_keys response is missing apiKey")
	}
	copied, err := h.glmBusiness(ctx, "GET", listURL+"/copy/"+url.PathEscape(key), headers, nil, "api key copy")
	if err != nil {
		return "", "", err
	}
	secret, _ := copied.(map[string]any)
	secretKey := firstNonEmptyString(secret["secretKey"])
	if secretKey == "" {
		return "", "", failText("Z.ai api key copy response is missing secretKey")
	}
	return key + "." + secretKey, token, nil
}
func (h *OAuthEngine) glmDevice(w http.ResponseWriter, r *http.Request) {
	fail := func(err error) { nodeReply(w, 500, map[string]string{"error": err.Error()}) }
	c := h.config("glm")
	if c == nil {
		fail(errDeviceNotSupported("glm"))
		return
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		fail(err)
		return
	}
	token := hex.EncodeToString(bytes)
	provider := c.ProviderID
	if provider == "" {
		provider = "zai"
	}
	raw, _ := json.Marshal(map[string]string{"provider": provider})
	status, payload, text, err := h.deviceJSON(r.Context(), "POST", c.CLIInitURL, map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + token}, strings.NewReader(string(raw)))
	if err != nil {
		fail(err)
		return
	}
	if status < 200 || status >= 300 {
		fail(failMessage("ZCode OAuth init failed", text))
		return
	}
	data, _ := payload["data"].(map[string]any)
	if !glmSuccess(payload["code"]) || data == nil {
		message := firstNonEmptyString(payload["msg"])
		if message == "" {
			message = "ZCode OAuth init returned no data"
		}
		fail(failText(message))
		return
	}
	if !nodeTruthy(data["flow_id"]) || !nodeTruthy(data["authorize_url"]) {
		fail(failText("ZCode OAuth init response missing flow_id/authorize_url"))
		return
	}
	expiration := float64(300)
	if timestamp, ok := data["expires_at"].(float64); ok && timestamp > 0 {
		if timestamp <= 1e12 {
			timestamp *= 1000
		}
		remaining := math.Floor((timestamp - float64(time.Now().UnixMilli())) / 1000)
		if remaining > 0 {
			expiration = remaining
		}
	}
	interval := data["poll_interval_sec"]
	if !nodeTruthy(interval) {
		interval = float64(3)
	}
	verifier, _, err := pkcePair()
	if err != nil {
		fail(err)
		return
	}
	nodeReply(w, 200, map[string]any{"device_code": data["flow_id"], "verification_uri": data["authorize_url"], "expires_in": expiration, "interval": interval, "_zcodePollToken": token, "codeVerifier": verifier})
}
func (h *OAuthEngine) glmPoll(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid or empty request body"})
		return
	}
	if !nodeTruthy(body["deviceCode"]) {
		nodeReply(w, 400, map[string]string{"error": "Missing device code"})
		return
	}
	replyError := func(code, desc string, pending bool) {
		value := map[string]any{"success": false, "error": code}
		if desc != "" {
			value["errorDescription"] = desc
		}
		if pending {
			value["pending"] = true
		}
		nodeReply(w, 200, value)
	}
	extra, _ := body["extraData"].(map[string]any)
	pollToken, _ := extra["_zcodePollToken"].(string)
	if pollToken == "" {
		replyError("access_denied", "Missing ZCode poll token — restart the login flow", false)
		return
	}
	c := h.config("glm")
	if c == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown provider: glm"})
		return
	}
	status, payload, _, err := h.deviceJSON(r.Context(), "GET", c.CLIPollURL+"/"+url.PathEscape(jsString(body["deviceCode"])), map[string]string{"Authorization": "Bearer " + pollToken}, nil)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if status < 200 || status >= 300 {
		replyError("access_denied", fmt.Sprintf("ZCode poll failed (HTTP %d)", status), false)
		return
	}
	if !glmSuccess(payload["code"]) {
		desc := firstNonEmptyString(payload["msg"])
		if desc == "" {
			desc = "ZCode poll failed"
		}
		replyError("access_denied", desc, false)
		return
	}
	data, _ := payload["data"].(map[string]any)
	state, _ := data["status"].(string)
	switch state {
	case "pending":
		replyError("authorization_pending", "", true)
		return
	case "failed":
		replyError("access_denied", "ZCode authorization failed or was cancelled", false)
		return
	case "ready":
	default:
		replyError("authorization_pending", "Unknown status: "+state, true)
		return
	}
	providerData, _ := data[c.ProviderID].(map[string]any)
	if providerData == nil {
		if id, ok := data["providerId"].(string); ok {
			providerData, _ = data[id].(map[string]any)
		}
	}
	access := firstNonEmptyString(providerData["access_token"], providerData["accessToken"], data["accessToken"], data["access_token"])
	if access == "" {
		replyError("access_denied", "ZCode poll response missing access token", false)
		return
	}
	key, business, err := h.glmPlanKey(r.Context(), c, access)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	user, _ := data["user"].(map[string]any)
	specific := map[string]any{"authMethod": "cli_poll"}
	for source, target := range map[string]string{"name": "username", "user_id": "userId"} {
		if nodeTruthy(user[source]) {
			specific[target] = user[source]
		}
	}
	if nodeTruthy(data["token"]) {
		specific["zcodeJwtToken"] = data["token"]
	}
	if business != "" {
		specific["zaiBusinessToken"] = business
	}
	if refresh := firstNonEmptyString(providerData["refresh_token"], providerData["refreshToken"], data["refresh_token"], data["refreshToken"]); refresh != "" {
		specific["zaiRefreshToken"] = refresh
	}
	mapped := map[string]any{"provider": "glm", "authType": "oauth", "accessToken": key, "refreshToken": nil, "email": nil, "expiresAt": nil, "testStatus": "active", "providerSpecificData": specific}
	if nodeTruthy(user["email"]) {
		mapped["email"] = user["email"]
	}
	if display := firstNonEmpty(user["name"], user["email"]); display != nil {
		mapped["displayName"] = display
	}
	saved, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"]}})
}

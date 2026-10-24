package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (h *OAuthEngine) deviceJSON(ctx context.Context, method, target string, headers map[string]string, body io.Reader) (int, map[string]any, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return 0, nil, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := h.client().Do(req)
	if err != nil {
		return 0, nil, nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return 0, nil, nil, err
	}
	var data map[string]any
	_ = json.Unmarshal(raw, &data)
	return res.StatusCode, data, raw, nil
}
func (h *OAuthEngine) kiloDevice(ctx context.Context) (map[string]any, error) {
	c := h.config("kilocode")
	if c == nil || c.InitiateURL == "" {
		return nil, errDeviceNotSupported("kilocode")
	}
	status, data, raw, err := h.deviceJSON(ctx, "POST", c.InitiateURL, map[string]string{"Content-Type": "application/json"}, nil)
	if err != nil {
		return nil, err
	}
	if status == 429 {
		return nil, failText("Too many pending authorization requests. Please try again later.")
	}
	if status < 200 || status >= 300 {
		return nil, failMessage("Device auth initiation failed", raw)
	}
	if data == nil {
		return nil, failText("Invalid device auth response")
	}
	result := map[string]any{"expires_in": 300, "interval": 3}
	for source, target := range map[string]string{"code": "device_code", "verificationUrl": "verification_uri"} {
		if value, ok := data[source]; ok {
			result[target] = value
		}
	}
	if value, ok := data["code"]; ok {
		result["user_code"] = value
	}
	if value, ok := data["verificationUrl"]; ok {
		result["verification_uri_complete"] = value
	}
	if nodeTruthy(data["expiresIn"]) {
		result["expires_in"] = data["expiresIn"]
	}
	return result, nil
}
func (h *OAuthEngine) kiloPoll(ctx context.Context, code string) (bool, map[string]any, error) {
	c := h.config("kilocode")
	if c == nil || c.PollURLBase == "" {
		return false, nil, errDeviceNotSupported("kilocode")
	}
	status, data, _, err := h.deviceJSON(ctx, "GET", c.PollURLBase+"/"+url.PathEscape(code), nil, nil)
	if err != nil {
		return false, nil, err
	}
	switch status {
	case 202:
		return false, map[string]any{"error": "authorization_pending"}, nil
	case 403:
		return false, map[string]any{"error": "access_denied", "error_description": "Authorization denied by user"}, nil
	case 410:
		return false, map[string]any{"error": "expired_token", "error_description": "Authorization code expired"}, nil
	}
	if status < 200 || status >= 300 {
		return false, map[string]any{"error": "poll_failed", "error_description": fmt.Sprintf("Poll failed: %d", status)}, nil
	}
	if data["status"] != "approved" || !nodeTruthy(data["token"]) {
		return false, map[string]any{"error": "authorization_pending"}, nil
	}
	token := jsString(data["token"])
	mapped := map[string]any{"access_token": token}
	if value, ok := data["userEmail"]; ok {
		mapped["_userEmail"] = value
	}
	if c.APIBaseURL != "" {
		status, profile, _, err := h.deviceJSON(ctx, "GET", strings.TrimSuffix(c.APIBaseURL, "/")+"/api/profile", map[string]string{"Authorization": "Bearer " + token}, nil)
		if err == nil && status >= 200 && status < 300 {
			if orgs, ok := profile["organizations"].([]any); ok && len(orgs) > 0 {
				if org, ok := orgs[0].(map[string]any); ok && nodeTruthy(org["id"]) {
					mapped["_orgId"] = org["id"]
				}
			}
		}
	}
	return true, mapped, nil
}
func kimiHeaders(deviceID string) map[string]string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x64"
	case "386":
		arch = "ia32"
	}
	model := runtime.GOOS + " " + arch
	switch runtime.GOOS {
	case "linux":
		model = "Linux " + arch
	case "darwin":
		model = "macOS " + arch
	case "windows":
		model = "Windows " + arch
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	if strings.TrimSpace(deviceID) == "" {
		deviceID = fmt.Sprintf("kimi-%d", time.Now().UnixMilli())
	}
	return map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json", "X-Msh-Platform": "9router", "X-Msh-Version": strings.TrimSpace(appVersion), "X-Msh-Device-Name": host, "X-Msh-Device-Model": model, "X-Msh-Device-Id": strings.TrimSpace(deviceID)}
}
func (h *OAuthEngine) kimiDevice(ctx context.Context) (map[string]any, error) {
	c := h.config("kimi")
	if c == nil {
		return nil, errDeviceNotSupported("kimi")
	}
	id := uuid.NewString()
	status, data, raw, err := h.deviceJSON(ctx, "POST", c.DeviceCodeURL, kimiHeaders(id), strings.NewReader(url.Values{"client_id": {c.ClientID}}.Encode()))
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, failMessage("Device code request failed", raw)
	}
	if data == nil {
		return nil, failText("Invalid device code response")
	}
	authorize := c.AuthorizeDeviceURL
	if authorize == "" {
		authorize = "https://www.kimi.com/code/authorize_device"
	}
	result := map[string]any{"interval": 5, "_kimiDeviceId": id, "verification_uri": authorize, "verification_uri_complete": authorize + "?user_code=" + jsString(data["user_code"])}
	for _, key := range []string{"device_code", "user_code", "expires_in"} {
		if value, ok := data[key]; ok {
			result[key] = value
		}
	}
	for _, key := range []string{"verification_uri", "verification_uri_complete", "interval"} {
		if nodeTruthy(data[key]) {
			result[key] = data[key]
		}
	}
	return result, nil
}
func (h *OAuthEngine) kimiPoll(ctx context.Context, code, deviceID string) (bool, map[string]any, error) {
	c := h.config("kimi")
	if c == nil {
		return false, nil, errDeviceNotSupported("kimi")
	}
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "client_id": {c.ClientID}, "device_code": {code}}
	status, data, _, err := h.deviceJSON(ctx, "POST", c.TokenURL, kimiHeaders(deviceID), strings.NewReader(form.Encode()))
	if err != nil {
		return false, nil, err
	}
	if data == nil {
		data = map[string]any{"error": "invalid_response", "error_description": "non-json token response"}
	}
	if nodeTruthy(data["access_token"]) && deviceID != "" {
		data["_kimiDeviceId"] = deviceID
	}
	return (status >= 200 && status < 300) || nodeTruthy(data["access_token"]) || nodeTruthy(data["error"]), data, nil
}
func kimiTokenMap(data map[string]any) map[string]any {
	mapped := defaultTokenMap(data)
	specific := map[string]any{"authMethod": "device_code"}
	if nodeTruthy(data["_kimiDeviceId"]) {
		specific["deviceId"] = data["_kimiDeviceId"]
	}
	mapped["providerSpecificData"] = specific
	return mapped
}
func kiloTokenMap(data map[string]any) map[string]any {
	mapped := map[string]any{"accessToken": data["access_token"], "refreshToken": nil, "expiresIn": nil}
	if value, ok := data["_userEmail"]; ok {
		mapped["email"] = value
	}
	if nodeTruthy(data["_orgId"]) {
		mapped["providerSpecificData"] = map[string]any{"orgId": data["_orgId"]}
	}
	return mapped
}

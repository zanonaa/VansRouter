package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

func zedOrgID(value any) string {
	if !nodeTruthy(value) {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []any:
		if len(v) > 0 {
			if s, ok := v[0].(string); ok {
				return s
			}
		}
	case map[string]any:
		if s, ok := v["0"].(string); ok {
			return s
		}
		if s, ok := v["id"].(string); ok {
			return s
		}
	}
	return jsString(value)
}
func zedOrganization(user map[string]any) string {
	if id := zedOrgID(firstNonEmpty(user["default_organization_id"], user["defaultOrganizationId"])); id != "" {
		return id
	}
	orgs, _ := user["organizations"].([]any)
	if len(orgs) == 0 {
		return ""
	}
	selected, _ := orgs[0].(map[string]any)
	for _, raw := range orgs {
		org, _ := raw.(map[string]any)
		if nodeTruthy(org["is_personal"]) {
			selected = org
			break
		}
	}
	return zedOrgID(selected["id"])
}
func (h *OAuthEngine) ZedImport(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid request body"})
		return
	}
	token, _ := body["accessToken"].(string)
	token = strings.TrimSpace(token)
	userID := ""
	if body["userId"] != nil {
		userID = strings.TrimSpace(jsString(body["userId"]))
	}
	systemID, _ := body["systemId"].(string)
	systemID = strings.TrimSpace(systemID)
	if systemID == "" {
		systemID = uuid.NewString()
	}
	if token == "" {
		nodeReply(w, 400, map[string]string{"error": "Access token is required"})
		return
	}
	if userID == "" {
		nodeReply(w, 400, map[string]string{"error": "User id is required"})
		return
	}
	target := "https://cloud.zed.dev/client/users/me"
	if c := h.config("zed"); c != nil && c.UserInfoURL != "" {
		target = c.UserInfoURL
	}
	status, user, raw, err := h.deviceJSON(r.Context(), "GET", target, map[string]string{"Accept": "application/json", "Authorization": userID + " " + token, "x-zed-system-id": systemID}, nil)
	if err != nil {
		nodeReply(w, 401, map[string]string{"error": "Zed token validation failed"})
		return
	}
	if status < 200 || status >= 300 {
		message := firstNonEmpty(user["message"])
		if message == nil {
			if nested, ok := user["error"].(map[string]any); ok {
				message = firstNonEmpty(nested["message"])
			} else {
				message = firstNonEmpty(user["error"])
			}
		}
		if message == nil {
			if len(raw) > 0 {
				message = string(raw)
			} else {
				message = fmt.Sprintf("HTTP %d", status)
			}
		}
		nodeReply(w, 401, map[string]string{"error": jsString(message)})
		return
	}
	email := firstNonEmpty(user["email"])
	display := firstNonEmpty(user["name"], user["display_name"], user["username"], "Zed "+userID)
	mapped := map[string]any{"provider": "zed", "authType": "oauth", "accessToken": token, "refreshToken": nil, "expiresAt": nil, "email": email, "displayName": display, "providerSpecificData": map[string]any{"authMethod": "imported", "userId": userID, "systemId": systemID, "organizationId": zedOrganization(user)}, "testStatus": "active"}
	saved, err := h.CreateConnection(r.Context(), mapped)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"], "email": saved["email"], "displayName": saved["displayName"]}})
}

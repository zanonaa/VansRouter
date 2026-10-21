package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func (h *OAuthEngine) CodexBulkImport(w http.ResponseWriter, r *http.Request) {
	var body any
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid JSON body: " + err.Error()})
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		nodeReply(w, 400, map[string]string{"error": "Invalid JSON body: trailing data"})
		return
	}
	var accounts []any
	switch value := body.(type) {
	case []any:
		accounts = value
	case map[string]any:
		if wrapped, ok := value["accounts"].([]any); ok {
			accounts = wrapped
		} else {
			accounts = []any{value}
		}
	}
	if len(accounts) == 0 {
		nodeReply(w, 400, map[string]string{"error": "No accounts provided"})
		return
	}
	results := []map[string]any{}
	success, failed := 0, 0
	for index, raw := range accounts {
		reject := func(message string) {
			failed++
			results = append(results, map[string]any{"index": index, "ok": false, "error": message})
		}
		input, ok := raw.(map[string]any)
		if !ok {
			reject("Item is not an object")
			continue
		}
		item := map[string]any{}
		for key, value := range input {
			switch key {
			case "id", "provider", "authType", "createdAt", "updatedAt":
			default:
				item[key] = value
			}
		}
		token, ok := item["accessToken"].(string)
		if !ok || token == "" {
			reject("Missing accessToken")
			continue
		}
		specific := jsSpread(item["providerSpecificData"])
		if !nodeTruthy(item["email"]) || !nodeTruthy(specific["chatgptAccountId"]) || !nodeTruthy(specific["chatgptPlanType"]) {
			jwt := token
			if idToken, ok := item["idToken"].(string); ok && idToken != "" {
				jwt = idToken
			}
			identity := map[string]any{}
			applyCodexClaims(identity, decodeJWTClaims(jwt), nil)
			if !nodeTruthy(item["email"]) && nodeTruthy(identity["email"]) {
				item["email"] = identity["email"]
			}
			if data, ok := identity["providerSpecificData"].(map[string]any); ok {
				for key, value := range data {
					if !nodeTruthy(specific[key]) {
						specific[key] = value
					}
				}
			}
		}
		if len(specific) > 0 {
			item["providerSpecificData"] = specific
		}
		if !nodeTruthy(item["expiresAt"]) {
			if seconds, ok := item["expiresIn"].(float64); ok && seconds > 0 {
				item["expiresAt"] = time.Now().Add(time.Duration(seconds * float64(time.Second))).UTC().Format("2006-01-02T15:04:05.000Z")
			}
		}
		if _, exists := item["testStatus"]; !exists {
			item["testStatus"] = "active"
		}
		if _, exists := item["isActive"]; !exists {
			item["isActive"] = true
		}
		if !nodeTruthy(item["lastRefreshAt"]) {
			item["lastRefreshAt"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		}
		item["provider"] = "codex"
		item["authType"] = "oauth"
		saved, err := h.CreateConnection(r.Context(), item)
		if err != nil {
			reject("Failed to save connection")
			continue
		}
		success++
		results = append(results, map[string]any{"index": index, "ok": true, "id": saved["id"]})
	}
	nodeReply(w, 200, map[string]any{"success": success, "failed": failed, "results": results})
}

package handlers

import (
	"net/http"
	"strings"
)

// CodexImportToken importa credencial fornecida pelo operador autenticado.
// Claims são metadados não verificados, não uma autorização do dashboard.
func (h *OAuthEngine) CodexImportToken(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid request body"})
		return
	}
	access, ok := body["accessToken"].(string)
	if !ok || access == "" {
		nodeReply(w, 400, map[string]string{"error": "Access token is required"})
		return
	}
	token := strings.TrimSpace(access)
	specific := map[string]any{"authMethod": "access_token"}
	var email any
	claims := decodeJWTClaims(token)
	if claims != nil {
		auth, _ := claims["https://api.openai.com/auth"].(map[string]any)
		profile, _ := claims["https://api.openai.com/profile"].(map[string]any)
		email = firstNonEmpty(profile["email"], claims["email"], claims["preferred_username"])
		for source, target := range map[string]string{"chatgpt_account_id": "chatgptAccountId", "chatgpt_plan_type": "chatgptPlanType"} {
			if nodeTruthy(auth[source]) {
				specific[target] = auth[source]
			}
		}
		if nodeTruthy(claims["exp"]) {
			specific["jwtExp"] = claims["exp"]
		}
	}
	if !nodeTruthy(email) {
		fallback := map[string]any{}
		applyCodexClaims(fallback, claims, nil)
		if nodeTruthy(fallback["email"]) {
			email = fallback["email"]
		}
		if data, ok := fallback["providerSpecificData"].(map[string]any); ok {
			for key, value := range data {
				specific[key] = value
			}
		}
	}
	name := firstNonEmpty(body["name"], email, "ChatGPT Access Token")
	saved, err := h.CreateConnection(r.Context(), map[string]any{"provider": "codex", "authType": "access_token", "accessToken": token, "name": name, "email": email, "providerSpecificData": specific, "testStatus": "active"})
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	var workspace, plan any
	if nodeTruthy(specific["chatgptAccountId"]) {
		workspace = specific["chatgptAccountId"]
	}
	if nodeTruthy(specific["chatgptPlanType"]) {
		plan = specific["chatgptPlanType"]
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"], "email": saved["email"], "name": saved["name"], "workspace": workspace, "plan": plan}})
}

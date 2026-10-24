package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

// GrokBulkImport recebe credenciais do operador autenticado. Claims JWT são
// apenas metadados; não autorizam o acesso ao dashboard.
func (h *OAuthEngine) GrokBulkImport(w http.ResponseWriter, r *http.Request) {
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
	for index, value := range accounts {
		reject := func(message string) {
			failed++
			results = append(results, map[string]any{"index": index, "ok": false, "error": message})
		}
		raw, ok := value.(map[string]any)
		if !ok {
			reject("Item is not an object")
			continue
		}
		access := firstNonEmpty(raw["access_token"], raw["accessToken"])
		token, ok := access.(string)
		if !ok || token == "" {
			reject("Missing access_token / accessToken")
			continue
		}
		refresh := firstNonEmpty(raw["refresh_token"], raw["refreshToken"])
		idToken := firstNonEmpty(raw["id_token"], raw["idToken"])
		email := firstNonEmpty(raw["email"])
		if !nodeTruthy(email) {
			if encoded, ok := idToken.(string); ok {
				email = firstNonEmpty(decodeJWTClaims(encoded)["email"])
			}
			if !nodeTruthy(email) {
				email = firstNonEmpty(decodeJWTClaims(token)["email"])
			}
		}
		expiresAt := firstNonEmpty(raw["expires_at"], raw["expiresAt"])
		expiresIn := firstNonEmpty(raw["expires_in"], raw["expiresIn"])
		if expiresAt == nil {
			if seconds, ok := expiresIn.(float64); ok && seconds > 0 {
				expiresAt = time.Now().Add(time.Duration(seconds * float64(time.Second))).UTC().Format("2006-01-02T15:04:05.000Z")
			}
		}
		specific := map[string]any{"authMethod": "device_code"}
		if idToken != nil {
			specific["idToken"] = idToken
		}
		if email != nil {
			specific["email"] = email
		}
		for key, v := range jsSpread(raw["providerSpecificData"]) {
			specific[key] = v
		}
		data := map[string]any{"provider": "grok-cli", "authType": "oauth", "accessToken": token, "refreshToken": refresh, "expiresAt": expiresAt, "email": email, "providerSpecificData": specific, "testStatus": "active"}
		if display := firstNonEmpty(raw["displayName"], raw["name"]); display != nil {
			data["displayName"] = display
		}
		created, err := h.CreateConnection(r.Context(), data)
		if err != nil {
			reject("Failed to save connection")
			continue
		}
		success++
		result := map[string]any{"index": index, "ok": true, "id": created["id"]}
		if email, exists := created["email"]; exists {
			result["email"] = email
		}
		results = append(results, result)
	}
	nodeReply(w, 200, map[string]any{"total": len(accounts), "success": success, "failed": failed, "results": results})
}

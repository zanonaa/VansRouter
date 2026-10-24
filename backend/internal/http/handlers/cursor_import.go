package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf16"
)

var cursorMachineFormat = regexp.MustCompile(`(?i)^[a-f0-9-]{32,}$`)

func (h *OAuthEngine) CursorImport(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 500, map[string]string{"error": "Invalid request body"})
		return
	}
	token, ok := body["accessToken"].(string)
	if !ok || token == "" {
		nodeReply(w, 400, map[string]string{"error": "Access token is required"})
		return
	}
	machine, ok := body["machineId"].(string)
	if !ok || machine == "" {
		nodeReply(w, 400, map[string]string{"error": "Machine ID is required"})
		return
	}
	token = strings.TrimSpace(token)
	machine = strings.TrimSpace(machine)
	fail := func(message string) { nodeReply(w, 500, map[string]string{"error": message}) }
	if token == "" {
		fail("Access token is required")
		return
	}
	if machine == "" {
		fail("Machine ID is required")
		return
	}
	if len(utf16.Encode([]rune(token))) < 50 {
		fail("Invalid token format. Token appears too short.")
		return
	}
	if !cursorMachineFormat.MatchString(strings.ReplaceAll(machine, "-", "")) {
		fail("Invalid machine ID format. Expected UUID format.")
		return
	}
	claims := decodeJWTClaims(token)
	email := firstNonEmpty(claims["email"], claims["sub"])
	userID := firstNonEmpty(claims["sub"], claims["user_id"])
	specific := map[string]any{"machineId": machine, "authMethod": "imported", "provider": "Imported"}
	if userID != nil {
		specific["userId"] = userID
	}
	saved, err := h.CreateConnection(r.Context(), map[string]any{"provider": "cursor", "authType": "oauth", "accessToken": token, "refreshToken": nil, "expiresAt": time.Now().Add(24 * time.Hour).UTC().Format("2006-01-02T15:04:05.000Z"), "email": email, "providerSpecificData": specific, "testStatus": "active"})
	if err != nil {
		fail("Failed to save connection")
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{"id": saved["id"], "provider": saved["provider"], "email": saved["email"]}})
}
func CursorInstructions(w http.ResponseWriter, r *http.Request) {
	nodeReply(w, 200, map[string]any{"provider": "cursor", "method": "import_token", "instructions": map[string]any{"title": "How to get your Cursor token", "steps": []string{
		"1. Open Cursor IDE and make sure you're logged in",
		"2. Find the state.vscdb file:",
		"   - Linux: ~/.config/Cursor/User/globalStorage/state.vscdb",
		"   - macOS: /Users/<user>/Library/Application Support/Cursor/User/globalStorage/state.vscdb",
		"   - Windows: %APPDATA%\\Cursor\\User\\globalStorage\\state.vscdb",
		"3. Open the database with SQLite browser or CLI:",
		"   sqlite3 state.vscdb \"SELECT value FROM itemTable WHERE key='cursorAuth/accessToken'\"",
		"4. Also get the machine ID:",
		"   sqlite3 state.vscdb \"SELECT value FROM itemTable WHERE key='storage.serviceMachineId'\"",
		"5. Paste both values in the form below",
	}, "alternativeMethod": []string{"Or use this one-liner to get both values:", "sqlite3 state.vscdb \"SELECT key, value FROM itemTable WHERE key IN ('cursorAuth/accessToken', 'storage.serviceMachineId')\""}}, "requiredFields": []map[string]string{
		{"name": "accessToken", "label": "Access Token", "description": "From cursorAuth/accessToken in state.vscdb", "type": "textarea"},
		{"name": "machineId", "label": "Machine ID", "description": "From storage.serviceMachineId in state.vscdb", "type": "text"},
	}})
}

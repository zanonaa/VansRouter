package auth

import (
	"encoding/json"
	"errors"
	"net/http"
)

// safeSettings reproduz a resposta de settings sem segredos.
func safeSettings(settings map[string]any) map[string]any {
	safe := make(map[string]any, len(settings))
	for key, value := range settings {
		if key == "password" || key == "oidcClientSecret" {
			continue
		}
		safe[key] = value
	}
	safe["oidcConfigured"] = configured(settings)
	safe["hasPassword"] = text(settings, "password") != ""
	return safe
}

// SettingsGet reproduz GET /api/settings.
func (h *HTTP) SettingsGet(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	settings, err := h.Settings(r.Context())
	if err != nil {
		// O original responde error.message; mensagem fixa é divergência documentada.
		reply(w, 500, map[string]string{"error": "Failed to load settings"})
		return
	}
	safe := safeSettings(settings)
	safe["enableRequestLogs"] = h.EnableRequestLogs
	safe["enableTranslator"] = h.EnableTranslator
	reply(w, 200, safe)
}

func decodeBody(r *http.Request) (map[string]any, error) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}
	if body == nil { // literal null
		return nil, errors.New("corpo nulo")
	}
	return body, nil
}

const (
	mustChangeOnly        = "Password change required: set only newPassword and currentPassword"
	weakPasswordMessage   = "New password must be at least 12 characters and not a placeholder"
	invalidCurrentMessage = "Invalid current password"
)

const notMigratedMessage = "Only the password change is migrated; other settings updates are pending"

// jsTruthy replica a semântica booleana do JavaScript para os valores JSON usados.
func jsTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case float64:
		return v != 0
	default:
		return true
	}
}

// grantChangeAllowed reproduz a validação do caminho com grant do original.
// Um retorno resume a recusa: código e mensagem.
func grantChangeAllowed(body map[string]any, initial string) (int, string) {
	keys := 0
	rawCurrent, hasCurrent := body["currentPassword"]
	for key := range body {
		if key == "password" {
			continue // protegida (CWE-915); será removida adiante
		}
		keys++
		if key != "newPassword" && key != "currentPassword" {
			return http.StatusForbidden, mustChangeOnly
		}
	}
	if keys == 0 || !hasCurrent || !jsTruthy(rawCurrent) {
		return http.StatusForbidden, mustChangeOnly
	}
	// O original compara com getInitialPassword() com igualdade estrita;
	// valor não string é sempre recusado (como !== string no JS).
	current, ok := rawCurrent.(string)
	if !ok || current != initial {
		return http.StatusUnauthorized, invalidCurrentMessage
	}
	return 0, ""
}

// currentPasswordOf extrai o currentPassword para comparação bcrypt.
func currentPasswordOf(body map[string]any) (present, isString bool, value string) {
	raw, ok := body["currentPassword"]
	if !ok || !jsTruthy(raw) {
		return false, false, ""
	}
	value, isString = raw.(string)
	return true, isString, value
}

// SettingsPatch reproduz PATCH /api/settings restrito à troca de senha.
// Escopo deste lote: mutações de outras chaves são recusadas com 501 porque os
// efeitos colaterais do original (proxy de saída, rotação de combos, autoPing,
// gravação de oidcClientSecret) ainda não migraram — nada é gravado em silêncio.
func (h *HTTP) SettingsPatch(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	body, err := decodeBody(r)
	if err != nil {
		// O original devolve error.message do parser; mensagem fixa é divergência documentada.
		reply(w, 500, map[string]string{"error": "Failed to parse settings payload"})
		return
	}
	settings, err := h.Settings(r.Context())
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to load settings"})
		return
	}
	session := h.session(r)
	grant := session == nil && h.grantOnly(r)
	if grant {
		if status, message := grantChangeAllowed(body, h.Initial); status != 0 {
			reply(w, status, map[string]string{"error": message})
			return
		}
		if _, ok := body["newPassword"]; !ok {
			// Grant exige a troca explícita da senha neste lote.
			reply(w, 403, map[string]string{"error": mustChangeOnly})
			return
		}
	} else if session == nil {
		// Defesa em profundidade: o guard já recusa, mas o handler exige sessão ou grant.
		reply(w, 401, map[string]string{"error": "Unauthorized"})
		return
	}

	// Mutações além da troca de senha não migram neste lote: recusa explícita.
	for key := range body {
		if key == "password" || key == "newPassword" || key == "currentPassword" {
			continue
		}
		reply(w, 501, map[string]string{"error": notMigratedMessage})
		return
	}

	delete(body, "password")
	var newPassword string
	if raw, ok := body["newPassword"]; ok && jsTruthy(raw) {
		value, isString := raw.(string)
		if !isString {
			// No original, todo valor truthy não string é tratado como fraco.
			reply(w, 400, map[string]string{"error": weakPasswordMessage})
			return
		}
		newPassword = value
	}
	if newPassword == "" {
		// Corpo sem nova senha; o original grava currentPassword como setting —
		// recusado/grant barrado aqui porque escritas gerais não migram neste lote.
		if grant {
			reply(w, 403, map[string]string{"error": mustChangeOnly})
			return
		}
		// Sessão: devolve a leitura atual sem incrementar revision (divergência menor documentada).
		reply(w, 200, safeSettings(settings))
		return
	}
	if !StrongPassword(newPassword) {
		reply(w, 400, map[string]string{"error": weakPasswordMessage})
		return
	}
	currentHash := text(settings, "password")
	present, isString, currentPassword := currentPasswordOf(body)
	if currentHash != "" {
		if !present {
			reply(w, 400, map[string]string{"error": "Current password required"})
			return
		}
		// O original chama bcrypt.compare; valor não string daria 500 — aqui é mismatch documentado.
		if !isString || !VerifyPassword(currentPassword, currentHash, "") {
			reply(w, 401, map[string]string{"error": invalidCurrentMessage})
			return
		}
	} else if present && (!isString || currentPassword != "123456") {
		// Primeira definição: valor legado qualquer é recusado (!== "123456" no original).
		reply(w, 401, map[string]string{"error": invalidCurrentMessage})
		return
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to hash password"})
		return
	}
	if _, err := h.ChangeP(r.Context(), hash); err != nil {
		reply(w, 500, map[string]string{"error": "Failed to update settings"})
		return
	}
	settings["password"] = hash
	reply(w, 200, safeSettings(settings))
}

// ResetPassword reproduz POST /api/auth/reset-password; a barreira local-only é do guard.
func (h *HTTP) ResetPassword(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	body, err := decodeBody(r)
	if err != nil {
		reply(w, 400, map[string]string{"error": "Invalid JSON body"})
		return
	}
	newPassword, _ := body["newPassword"].(string)
	if _, ok := body["newPassword"]; ok && jsTruthy(body["newPassword"]) {
		if value, isString := body["newPassword"].(string); !isString {
			// Mesmo tratamento de tipo da troca via sessão.
			reply(w, 400, map[string]string{"error": weakPasswordMessage})
			return
		} else {
			newPassword = value
		}
	}
	if !StrongPassword(newPassword) {
		reply(w, 400, map[string]string{"error": weakPasswordMessage})
		return
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to hash password"})
		return
	}
	if _, err := h.ChangeP(r.Context(), hash); err != nil {
		reply(w, 500, map[string]string{"error": "Failed to update settings"})
		return
	}
	reply(w, 200, map[string]bool{"success": true})
}

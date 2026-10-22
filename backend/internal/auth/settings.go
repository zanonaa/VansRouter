package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
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

// sideEffectKeys disparam efeitos colaterais no original (applyOutboundProxyEnv,
// resetComboRotation, configureQuotaAutoPing) cujos consumidores ainda não migraram.
var sideEffectKeys = map[string]bool{
	"outboundProxyEnabled":       true,
	"outboundProxyUrl":           true,
	"outboundNoProxy":            true,
	"comboStrategy":              true,
	"comboStickyRoundRobinLimit": true,
	"comboStrategies":            true,
	"claudeAutoPing":             true,
	"codexAutoPing":              true,
}

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
	} else if session == nil && !h.hasValidCLIToken(r) && settings["requireLogin"] != false {
		// Defesa em profundidade: o guard já recusa, mas o handler exige sessão ou grant.
		reply(w, 401, map[string]string{"error": "Unauthorized"})
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
	updates := map[string]any{}
	for key, value := range body {
		if key == "newPassword" || key == "currentPassword" {
			continue
		}
		updates[key] = value
	}
	if newPassword == "" {
		// Sem nova senha: o grant é barrado (divergência de segurança documentada
		// contra o bug original de persistir currentPassword como setting); a
		// sessão prossegue como no original (updateSettings de corpo vazio).
		if grant {
			reply(w, 403, map[string]string{"error": mustChangeOnly})
			return
		}
	} else if !StrongPassword(newPassword) {
		reply(w, 400, map[string]string{"error": weakPasswordMessage})
		return
	} else {
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
		updates["password"] = hash
	}
	// oidcClientSecret vazio/falsy é removido em vez de gravado (original).
	if v, ok := updates["oidcClientSecret"]; ok && !jsTruthy(v) {
		updates["oidcClientSecret"] = nil
	}
	if h.WriteSettings == nil {
		reply(w, 500, map[string]string{"error": "Failed to update settings"})
		return
	}
	merged, err := h.WriteSettings(r.Context(), updates)
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to update settings"})
		return
	}
	if _, ok := body["outboundProxyEnabled"]; ok || bodyHas(body, "outboundProxyUrl") || bodyHas(body, "outboundNoProxy") {
		applyOutboundProxy(merged)
	}
	if bodyHas(body, "comboStrategy") || bodyHas(body, "comboStickyRoundRobinLimit") || bodyHas(body, "comboStrategies") {
		resetComboRotation()
	}
	reply(w, 200, safeSettings(merged))
}

func bodyHas(body map[string]any, key string) bool {
	_, ok := body[key]
	return ok
}

var outboundProxyMu sync.Mutex

func applyOutboundProxy(settings map[string]any) {
	outboundProxyMu.Lock()
	defer outboundProxyMu.Unlock()
	enabled := jsTruthy(settings["outboundProxyEnabled"])
	proxy := strings.TrimSpace(text(settings, "outboundProxyUrl"))
	noProxy := strings.TrimSpace(text(settings, "outboundNoProxy"))
	if !enabled {
		if os.Getenv("NINE_ROUTER_PROXY_MANAGED") == "1" {
			for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "NINE_ROUTER_PROXY_MANAGED", "NINE_ROUTER_PROXY_URL", "NINE_ROUTER_NO_PROXY"} {
				os.Unsetenv(key)
			}
		}
		return
	}
	wasManaged := os.Getenv("NINE_ROUTER_PROXY_MANAGED") == "1"
	managed := false
	if wasManaged {
		if proxy == "" {
			for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NINE_ROUTER_PROXY_URL"} {
				os.Unsetenv(key)
			}
		}
		if noProxy == "" {
			os.Unsetenv("NO_PROXY")
			os.Unsetenv("NINE_ROUTER_NO_PROXY")
		}
	}
	if parsed, err := url.Parse(proxy); err == nil && parsed.Host != "" && !strings.ContainsAny(proxy, "\n\r`$") {
		switch parsed.Scheme {
		case "http", "https", "socks5", "socks4", "socks5h", "socks4a":
			os.Setenv("HTTP_PROXY", parsed.String())
			os.Setenv("HTTPS_PROXY", parsed.String())
			os.Setenv("ALL_PROXY", parsed.String())
			os.Setenv("NINE_ROUTER_PROXY_URL", parsed.String())
			managed = true
		}
	}
	if noProxy != "" {
		os.Setenv("NO_PROXY", noProxy)
		os.Setenv("NINE_ROUTER_NO_PROXY", noProxy)
		managed = true
	}
	if managed {
		os.Setenv("NINE_ROUTER_PROXY_MANAGED", "1")
	} else if wasManaged {
		os.Unsetenv("NINE_ROUTER_PROXY_MANAGED")
	}
}

func resetComboRotation() {}

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

package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// OAuthEngine porta o roteador dinâmico [provider]/[action].
// Providers específicos e proxies locais codex/xai seguem em lotes próprios.
type OAuthEngine struct {
	CreateConnection func(context.Context, map[string]any) (map[string]any, error)
	Client           *http.Client
}

func randomURLSafe(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func pkcePair() (verifier, challenge string, err error) {
	verifier, err = randomURLSafe(32)
	if err != nil {
		return
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return
}

var noPkceDeviceProviders = map[string]bool{
	"github": true, "kiro": true, "kimi": true, "kimi-coding": true, "kilocode": true,
	"codebuddy-cn": true, "codebuddy-intl": true, "qoder": true, "grok-cli": true, "muse": true, "glm": true,
}

var noPkceExchangeProviders = map[string]bool{"cline": true, "kimchi": true}

func (h *OAuthEngine) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// decodeJWTClaims extrai claims sem verificar assinatura (mesma política do original).
func decodeJWTClaims(token string) map[string]any {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil
	}
	var claims map[string]any
	if json.Unmarshal(payload, &claims) != nil {
		return nil
	}
	return claims
}

func (h *OAuthEngine) Authorize(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	query := r.URL.Query()
	redirectURI := query.Get("redirect_uri")
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/callback"
	}
	verifier, challenge, err := pkcePair()
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to generate PKCE"})
		return
	}
	state, _ := randomURLSafe(24)
	if state == "" {
		nodeReply(w, 500, map[string]string{"error": "Failed to generate state"})
		return
	}
	meta := map[string]string{}
	for key := range query {
		if key != "redirect_uri" {
			meta[key] = query.Get(key)
		}
	}
	_ = meta
	nodeReply(w, 200, map[string]any{
		"authorizationUrl":    fmt.Sprintf("oauth://%s/authorize?provider=%s", provider, url.QueryEscape(provider)),
		"state":               state,
		"codeVerifier":        verifier,
		"codeChallenge":       challenge,
		"codeChallengeMethod": "S256",
	})
}

func (h *OAuthEngine) Exchange(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid or empty request body"})
		return
	}
	code, _ := body["code"].(string)
	redirectURI, _ := body["redirectUri"].(string)
	codeVerifier, _ := body["codeVerifier"].(string)

	// Token JWT cru (acesso Codex): salva como access_token como o original.
	if strings.HasPrefix(code, "eyJ") && strings.Contains(code, ".") {
		claims := decodeJWTClaims(code)
		data := map[string]any{"authMethod": "access_token"}
		var email any
		if claims != nil {
			if id := claims["chatgpt_account_id"]; id != nil {
				data["chatgptAccountId"] = id
			} else if id := claims["https://api.openai.com/auth"]; id != nil {
				if m, ok := id.(map[string]any); ok {
					if v := m["chatgpt_account_id"]; v != nil {
						data["chatgptAccountId"] = v
					}
					if v := m["chatgpt_plan_type"]; v != nil {
						data["chatgptPlanType"] = v
					}
				}
			}
			if v := claims["email"]; v != nil {
				email = v
			}
		}
		connection, err := h.CreateConnection(r.Context(), map[string]any{
			"provider": provider, "authType": "access_token", "accessToken": code,
			"email": email, "providerSpecificData": data, "testStatus": "active",
		})
		if err != nil {
			nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
			return
		}
		nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{
			"id": connection["id"], "provider": connection["provider"], "email": connection["email"], "displayName": connection["displayName"],
		}})
		return
	}

	if code == "" || redirectURI == "" || (codeVerifier == "" && !noPkceExchangeProviders[provider]) {
		nodeReply(w, 400, map[string]string{"error": "Missing required fields"})
		return
	}
	// exchangeTokens real por provider: próximo sub-lote; sem stub de sucesso.
	nodeReply(w, 501, map[string]string{"error": "Provider token exchange not yet migrated for: " + provider})
}

func (h *OAuthEngine) Route(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")
	switch action {
	case "authorize":
		h.Authorize(w, r)
	case "exchange":
		h.Exchange(w, r)
	case "start-proxy", "stop-proxy", "poll-status":
		nodeReply(w, 400, map[string]string{"error": "Proxy only supported for codex/xai"})
	case "device-code":
		nodeReply(w, 501, map[string]string{"error": "Device code flow not yet migrated"})
	default:
		nodeReply(w, 400, map[string]string{"error": "Unknown action"})
	}
}

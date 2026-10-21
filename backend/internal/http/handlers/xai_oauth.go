package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Espelho de src/lib/oauth/constants/xai.js (fonte: CLIProxyAPI internal/auth/xai).
const (
	xaiClientID            = "b1a00492-073a-47ea-816f-4c329264a828"
	xaiIssuer              = "https://auth.x.ai"
	xaiAuthorizeFallback   = xaiIssuer + "/oauth2/authorize"
	xaiTokenFallback       = xaiIssuer + "/oauth2/token"
	xaiDiscoveryURL        = xaiIssuer + "/.well-known/openid-configuration"
	xaiScope               = "openid profile email offline_access grok-cli:access api:access"
	xaiCallbackPath        = "/callback"
	xaiCodeChallengeMethod = "S256"
	xaiLoopbackPort        = 56121
	// Verificador em bytes antes do base64url (96 bytes → 128 caracteres).
	xaiPKCEVerifierBytes = 96
)

type xaiEndpoints struct{ AuthorizeURL, TokenURL string }

var (
	xaiDiscoveryMu    sync.Mutex
	xaiDiscoveryCache *xaiEndpoints
)

func randomBase64URL(n int) (string, error) {
	value, err := randomURLSafe(n)
	if err != nil {
		return "", err
	}
	if _, err := base64.RawURLEncoding.DecodeString(value); err != nil {
		return "", err
	}
	return value, nil
}

func randomHex(n int) (string, error) {
	raw, err := randomURLSafe(n)
	if err != nil {
		return "", err
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", err
	}
	output := make([]byte, 0, len(decoded)*2)
	const hexDigits = "0123456789abcdef"
	for _, b := range decoded {
		output = append(output, hexDigits[b>>4], hexDigits[b&0x0f])
	}
	return string(output), nil
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// validateOAuthEndpoint espelha o guard do services/xai.js: somente https em *.x.ai.
func validateOAuthEndpoint(rawURL, field string) (string, error) {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return "", fmt.Errorf("xai discovery %s is empty", field)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		// O Node nunca falha em new URL(string); o texto do Go é a divergência
		// registrada para entradas não analisáveis.
		return "", fmt.Errorf("xai discovery %s is invalid: %v", field, err)
	}
	if parsed.Scheme != "https" {
		return "", fmt.Errorf("xai discovery %s must use https: %s", field, value)
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host != "x.ai" && !strings.HasSuffix(host, ".x.ai") {
		return "", fmt.Errorf("xai discovery %s host %s is not on x.ai", field, host)
	}
	return value, nil
}

// discoverXaiEndpoints consulta o discovery com cache de processo; qualquer
// falha (não 2xx, payload inválido, endpoint fora do domínio) reverte para os
// valores estáticos, como o try/catch da origem.
func (h *OAuthEngine) discoverXaiEndpoints(ctx context.Context) xaiEndpoints {
	xaiDiscoveryMu.Lock()
	defer xaiDiscoveryMu.Unlock()
	if xaiDiscoveryCache != nil {
		return *xaiDiscoveryCache
	}
	fallback := xaiEndpoints{AuthorizeURL: xaiAuthorizeFallback, TokenURL: xaiTokenFallback}
	req, err := http.NewRequestWithContext(ctx, "GET", xaiDiscoveryURL, nil)
	if err != nil {
		xaiDiscoveryCache = &fallback
		return fallback
	}
	req.Header.Set("Accept", "application/json")
	res, err := h.client().Do(req)
	if err != nil {
		xaiDiscoveryCache = &fallback
		return fallback
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		var data struct {
			AuthorizationEndpoint string `json:"authorization_endpoint"`
			TokenEndpoint         string `json:"token_endpoint"`
		}
		if err := json.Unmarshal(body, &data); err == nil {
			authorize, errAuthorize := validateOAuthEndpoint(data.AuthorizationEndpoint, "authorization_endpoint")
			token, errToken := validateOAuthEndpoint(data.TokenEndpoint, "token_endpoint")
			if errAuthorize == nil && errToken == nil {
				endpoints := xaiEndpoints{AuthorizeURL: authorize, TokenURL: token}
				xaiDiscoveryCache = &endpoints
				return endpoints
			}
		}
	}
	xaiDiscoveryCache = &fallback
	return fallback
}

// decodeXaiIDTokenEmail espelha decodeIdTokenEmail sem verificação de assinatura.
func decodeXaiIDTokenEmail(idToken string) string {
	claims := decodeJWTClaims(idToken)
	if claims == nil {
		return ""
	}
	for _, key := range []string{"email", "preferred_username", "sub"} {
		if nodeTruthy(claims[key]) {
			return jsString(claims[key])
		}
	}
	return ""
}

// buildXaiAuthURL inclui nonce/plan/referrer, espelhando BuildAuthorizeURL.
func buildXaiAuthURL(ctx context.Context, h *OAuthEngine, redirectURI, state, codeChallenge string) string {
	endpoints := h.discoverXaiEndpoints(ctx)
	nonce, err := randomHex(16)
	if err != nil {
		nonce = ""
	}
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", xaiClientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", xaiScope)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", xaiCodeChallengeMethod)
	params.Set("state", state)
	params.Set("nonce", nonce)
	params.Set("plan", "generic")
	params.Set("referrer", "cli-proxy-api")
	return endpoints.AuthorizeURL + "?" + params.Encode()
}

func (h *OAuthEngine) xaiAuthorize(w http.ResponseWriter, r *http.Request, redirectURI string) {
	if h.config("xai") == nil {
		nodeReply(w, 500, map[string]string{"error": "Unknown OAuth provider: xai"})
		return
	}
	verifier, err := randomBase64URL(xaiPKCEVerifierBytes)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	challenge := pkceChallenge(verifier)
	state, err := randomBase64URL(32)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	authURL := buildXaiAuthURL(r.Context(), h, redirectURI, state, challenge)
	nodeReply(w, 200, map[string]any{
		"authUrl": authURL, "state": state, "codeVerifier": verifier,
		"codeChallenge": challenge, "redirectUri": redirectURI,
		"flowType": "authorization_code_pkce", "fixedPort": xaiLoopbackPort,
		"callbackPath": xaiCallbackPath,
	})
}

// exchangeXaiTokens é o POST form-urlencoded do cliente público PKCE (sem
// client_secret), apontando para o token endpoint descoberto.
func (h *OAuthEngine) exchangeXaiTokens(ctx context.Context, code, redirectURI, codeVerifier string) (map[string]any, error) {
	endpoints := h.discoverXaiEndpoints(ctx)
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", xaiClientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoints.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("xAI token exchange failed: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, err := h.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("xAI token exchange failed: %v", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("xAI token exchange failed: %s", string(body))
	}
	var data map[string]any
	if json.Unmarshal(body, &data) != nil {
		return nil, fmt.Errorf("xAI token exchange failed: %s", string(body))
	}
	return mapXaiTokens(data), nil
}

// mapXaiTokens espelha providers.js xai.mapTokens: e-mail derivado do id_token
// e providerSpecificData contendo apenas o idToken.
func mapXaiTokens(raw map[string]any) map[string]any {
	mapped := map[string]any{}
	if email := decodeXaiIDTokenEmail(stringField(raw, "id_token")); email != "" {
		mapped["email"] = email
	}
	if nodeTruthy(raw["id_token"]) {
		mapped["providerSpecificData"] = map[string]any{"idToken": raw["id_token"]}
	}
	for source, target := range map[string]string{"access_token": "accessToken", "refresh_token": "refreshToken", "expires_in": "expiresIn", "scope": "scope"} {
		if value, ok := raw[source]; ok && value != nil {
			mapped[target] = value
		}
	}
	return mapped
}

// xaiConnectionData monta o payload de createProviderConnection como a rota
// original: spread do mapped + expiresAt + testStatus.
func xaiConnectionData(tokenData map[string]any) map[string]any {
	connection := map[string]any{"provider": "xai", "authType": "oauth", "testStatus": "active"}
	for _, key := range []string{"accessToken", "refreshToken", "expiresIn", "scope", "email", "providerSpecificData"} {
		if value, ok := tokenData[key]; ok {
			connection[key] = value
		}
	}
	if seconds, ok := tokenData["expiresIn"].(float64); ok && seconds > 0 {
		connection["expiresAt"] = time.Now().Add(time.Duration(seconds) * time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
	} else {
		connection["expiresAt"] = nil
	}
	return connection
}

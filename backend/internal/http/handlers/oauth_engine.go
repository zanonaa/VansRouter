package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// OAuthEngine porta o roteador dinâmico [provider]/[action].
type OAuthEngine struct {
	CreateConnection func(context.Context, map[string]any) (map[string]any, error)
	Client           *http.Client
	Config           func(string) *providers.OAuthConfig
}

func (h *OAuthEngine) config(provider string) *providers.OAuthConfig {
	if h.Config != nil {
		return h.Config(provider)
	}
	return providers.GetOAuthConfig(provider)
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

var noPkceExchangeProviders = map[string]bool{"cline": true, "kimchi": true}

func (h *OAuthEngine) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

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

func (h *OAuthEngine) buildAuthURL(provider, redirectURI string) (string, string, string, error) {
	config := h.config(provider)
	if config == nil {
		return "", "", "", fmt.Errorf("Unknown OAuth provider: %s", provider)
	}
	verifier, challenge, err := pkcePair()
	if err != nil {
		return "", "", "", err
	}
	state, err := randomURLSafe(32)
	if err != nil {
		return "", "", "", err
	}
	if config.AuthorizeURL == "" {
		return "", "", "", fmt.Errorf("Provider %s has no authorize URL", provider)
	}
	params := url.Values{}
	params.Set("client_id", config.ClientID)
	params.Set("response_type", "code")
	params.Set("redirect_uri", redirectURI)
	if provider == "claude" {
		params.Set("code", "true")
	}
	if config.Scope != "" {
		params.Set("scope", config.Scope)
	}
	for key, value := range config.ExtraParams {
		params.Set(key, value)
	}
	if len(config.Scopes) > 0 {
		params.Set("scope", strings.Join(config.Scopes, " "))
	}
	if config.CodeChallengeMethod == "S256" {
		params.Set("code_challenge", challenge)
		params.Set("code_challenge_method", "S256")
	}
	params.Set("state", state)
	return config.AuthorizeURL + "?" + params.Encode(), state, verifier, nil
}

func (h *OAuthEngine) exchangeTokens(ctx context.Context, provider, code, redirectURI, codeVerifier, state string) (map[string]any, error) {
	config := h.config(provider)
	if config == nil {
		return nil, fmt.Errorf("Unknown OAuth provider: %s", provider)
	}
	if config.TokenURL == "" {
		return nil, fmt.Errorf("Provider %s has no token URL", provider)
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", config.ClientID)
	if config.ClientSecret != "" {
		form.Set("client_secret", config.ClientSecret)
	}
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	contentType := "application/x-www-form-urlencoded"
	payload := form.Encode()
	if provider == "claude" {
		parts := strings.Split(code, "#")
		authCode := parts[0]
		if len(parts) > 1 && parts[1] != "" {
			state = parts[1]
		}
		data := map[string]string{"code": authCode, "state": state, "grant_type": "authorization_code", "client_id": config.ClientID, "redirect_uri": redirectURI, "code_verifier": codeVerifier}
		encoded, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		payload = string(encoded)
		contentType = "application/json"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", config.TokenURL, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json")
	res, err := h.client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("Token exchange failed: %w", err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	var data map[string]any
	if json.Unmarshal(body, &data) != nil {
		data = map[string]any{}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message := fmt.Sprintf("Token exchange failed (%d)", res.StatusCode)
		if d, _ := data["error_description"].(string); d != "" {
			message = d
		} else if e, _ := data["error"].(string); e != "" {
			message = e
		}
		return nil, fmt.Errorf("%s", message)
	}
	return data, nil
}

func (h *OAuthEngine) Authorize(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	if provider != "claude" && provider != "codex" {
		if provider == "gemini-cli" || provider == "iflow" {
			uri := r.URL.Query().Get("redirect_uri")
			if uri == "" {
				uri = "http://localhost:8080/callback"
			}
			if provider == "gemini-cli" {
				h.geminiAuthorize(w, r, uri)
			} else {
				h.iflowAuthorize(w, r, uri)
			}
			return
		}
		nodeReply(w, 501, map[string]string{"error": "OAuth authorization not yet migrated for: " + provider})
		return
	}
	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		redirectURI = "http://localhost:8080/callback"
	}
	authURL, state, verifier, err := h.buildAuthURL(provider, redirectURI)
	if err != nil {
		nodeReply(w, 400, map[string]string{"error": err.Error()})
		return
	}
	sum := sha256.Sum256([]byte(verifier))
	config := h.config(provider)
	result := map[string]any{"authUrl": authURL, "state": state, "codeVerifier": verifier, "codeChallenge": base64.RawURLEncoding.EncodeToString(sum[:]), "redirectUri": redirectURI, "flowType": "authorization_code_pkce", "callbackPath": "/callback"}
	if config.FixedPort != 0 {
		result["fixedPort"] = config.FixedPort
	}
	if config.CallbackPath != "" {
		result["callbackPath"] = config.CallbackPath
	}
	nodeReply(w, 200, result)
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
	if provider == "gemini-cli" {
		h.geminiExchange(w, r)
		return
	}
	if provider == "iflow" {
		h.iflowExchange(w, r)
		return
	}
	if provider != "claude" && provider != "codex" {
		nodeReply(w, 501, map[string]string{"error": "Provider token exchange not yet migrated for: " + provider})
		return
	}
	state, _ := body["state"].(string)
	tokenData, err := h.exchangeTokens(r.Context(), provider, code, redirectURI, codeVerifier, state)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	var expiresAt any
	if expiresIn, ok := tokenData["expires_in"].(float64); ok && expiresIn > 0 {
		expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
	}
	connectionData := map[string]any{
		"provider": provider, "authType": "oauth", "testStatus": "active",
	}
	for source, target := range map[string]string{"access_token": "accessToken", "refresh_token": "refreshToken", "id_token": "idToken", "token_type": "tokenType", "scope": "scope", "email": "email"} {
		if v, ok := tokenData[source]; ok {
			connectionData[target] = v
		}
	}
	if provider == "codex" {
		connectionData["lastRefreshAt"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		applyCodexClaims(connectionData, decodeJWTClaims(stringField(tokenData, "id_token")), decodeJWTClaims(stringField(tokenData, "access_token")))
	}
	if expiresAt != nil {
		connectionData["expiresAt"] = expiresAt
	}
	if expiresIn, ok := tokenData["expires_in"].(float64); ok {
		connectionData["expiresIn"] = expiresIn
	}
	connection, err := h.CreateConnection(r.Context(), connectionData)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save connection"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "connection": map[string]any{
		"id": connection["id"], "provider": connection["provider"], "email": connection["email"], "displayName": connection["displayName"],
	}})
}

func (h *OAuthEngine) Route(w http.ResponseWriter, r *http.Request) {
	action := chi.URLParam(r, "action")
	if chi.URLParam(r, "provider") == "github" {
		if r.Method == "GET" && action == "device-code" {
			h.githubDevice(w, r)
			return
		}
		if r.Method == "POST" && action == "poll" {
			h.githubPoll(w, r)
			return
		}
	}
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

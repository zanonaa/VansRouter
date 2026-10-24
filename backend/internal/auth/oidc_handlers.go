package auth

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// redirect reproduz NextResponse.redirect (307, mesmo verbo preservado).
func redirect(w http.ResponseWriter, r *http.Request, target string) {
	w.Header().Set("Location", target)
	w.WriteHeader(http.StatusTemporaryRedirect)
}

// loginErrorRedirect monta /login?error=... no origin público.
func loginErrorRedirect(w http.ResponseWriter, r *http.Request, message string) {
	target := GetPublicOrigin(r) + "/login?error=" + url.QueryEscape(message)
	redirect(w, r, target)
}

func (h *HTTP) oidcCookie(name, value string, secure bool) *http.Cookie {
	return &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: oidcCookieMaxAgeSec}
}

func clearOidcCookies(w http.ResponseWriter) {
	for _, name := range []string{"oidc_state", "oidc_nonce", "oidc_code_verifier"} {
		http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1})
	}
}

func oidcSecure(r *http.Request, force bool) bool {
	return force || r.Header.Get("X-Forwarded-Proto") == "https"
}

// OIDCStart reproduz GET /api/auth/oidc/start.
func (h *HTTP) OIDCStart(w http.ResponseWriter, r *http.Request) {
	// Mitigação CSRF/prefetch: apenas navegações de documento de topo.
	secPurpose := strings.ToLower(r.Header.Get("Sec-Purpose"))
	purpose := strings.ToLower(r.Header.Get("Purpose"))
	if secPurpose == "prefetch" || purpose == "prefetch" {
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusForbidden, map[string]string{"error": "Prefetch not allowed"})
		return
	}
	secFetchMode := r.Header.Get("Sec-Fetch-Mode")
	if secFetchMode != "" && secFetchMode != "navigate" {
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusForbidden, map[string]string{"error": "Invalid request mode"})
		return
	}
	secFetchDest := r.Header.Get("Sec-Fetch-Dest")
	if secFetchDest != "" && secFetchDest != "document" {
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusForbidden, map[string]string{"error": "Invalid request destination"})
		return
	}
	settings, err := h.Settings(r.Context())
	if err != nil {
		loginErrorRedirect(w, r, "oidc_start_failed")
		return
	}
	config := OidcRuntime(settings)
	if config == nil {
		loginErrorRedirect(w, r, "oidc_not_configured")
		return
	}
	discovery, err := h.FetchOidcDiscovery(config.IssuerURL)
	if err != nil {
		loginErrorRedirect(w, r, err.Error())
		return
	}
	state, err := CreateOidcState()
	if err != nil {
		loginErrorRedirect(w, r, "oidc_start_failed")
		return
	}
	nonce, err := CreateOidcNonce()
	if err != nil {
		loginErrorRedirect(w, r, "oidc_start_failed")
		return
	}
	verifier, challenge, err := CreatePkcePair()
	if err != nil {
		loginErrorRedirect(w, r, "oidc_start_failed")
		return
	}
	origin := GetPublicOrigin(r)
	redirectURI := origin + "/api/auth/oidc/callback"
	authURL := BuildOidcAuthorizationURL(discovery.AuthorizationEndpoint, config.ClientID, redirectURI, config.Scopes, state, nonce, challenge)
	secure := oidcSecure(r, h.ForceSecure)
	http.SetCookie(w, h.oidcCookie("oidc_state", state, secure))
	http.SetCookie(w, h.oidcCookie("oidc_nonce", nonce, secure))
	http.SetCookie(w, h.oidcCookie("oidc_code_verifier", verifier, secure))
	redirect(w, r, authURL)
}

// OIDCCallback reproduz GET /api/auth/oidc/callback.
func (h *HTTP) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if errorParam := query.Get("error"); errorParam != "" {
		loginErrorRedirect(w, r, errorParam)
		return
	}
	code := query.Get("code")
	state := query.Get("state")
	if code == "" || state == "" {
		clearOidcCookies(w)
		loginErrorRedirect(w, r, "oidc_missing_code")
		return
	}
	storedStateCookie, errState := r.Cookie("oidc_state")
	storedNonceCookie, errNonce := r.Cookie("oidc_nonce")
	verifierCookie, errVerifier := r.Cookie("oidc_code_verifier")
	if errState != nil || errNonce != nil || errVerifier != nil || storedStateCookie.Value == "" || storedNonceCookie.Value == "" || verifierCookie.Value == "" || storedStateCookie.Value != state {
		clearOidcCookies(w)
		loginErrorRedirect(w, r, "oidc_invalid_state")
		return
	}
	settings, err := h.Settings(r.Context())
	if err != nil {
		clearOidcCookies(w)
		loginErrorRedirect(w, r, "oidc_not_configured")
		return
	}
	config := OidcRuntime(settings)
	if config == nil {
		clearOidcCookies(w)
		loginErrorRedirect(w, r, "oidc_not_configured")
		return
	}
	discovery, err := h.FetchOidcDiscovery(config.IssuerURL)
	if err != nil {
		clearOidcCookies(w)
		loginErrorRedirect(w, r, err.Error())
		return
	}
	discoveredIssuer := discovery.Issuer
	if discoveredIssuer == "" {
		discoveredIssuer = config.IssuerURL
	}
	redirectURI := GetPublicOrigin(r) + "/api/auth/oidc/callback"
	tokenData, err := h.ExchangeOidcCode(discovery.TokenEndpoint, config.ClientID, config.ClientSecret, code, redirectURI, verifierCookie.Value)
	if err != nil {
		clearOidcCookies(w)
		loginErrorRedirect(w, r, err.Error())
		return
	}
	idToken, _ := tokenData["id_token"].(string)
	if idToken == "" {
		clearOidcCookies(w)
		loginErrorRedirect(w, r, "OIDC provider did not return an id_token")
		return
	}
	payload, err := h.VerifyOidcIdToken(idToken, discoveredIssuer, config.ClientID, discovery.JWKSURI, storedNonceCookie.Value)
	if err != nil {
		clearOidcCookies(w)
		loginErrorRedirect(w, r, err.Error())
		return
	}
	clearOidcCookies(w)
	sub, _ := payload["sub"].(string)
	email := PickOidcEmail(payload)
	name := PickOidcDisplayName(payload)
	claims := map[string]any{"oidc": true, "oidcSub": sub, "oidcName": name}
	if email != "" {
		claims["oidcEmail"] = email
	}
	token, err := h.Sessions.Create(claims)
	if err != nil {
		loginErrorRedirect(w, r, "oidc_callback_failed")
		return
	}
	http.SetCookie(w, SessionCookie(r, token, false, h.ForceSecure))
	redirect(w, r, GetPublicOrigin(r)+"/dashboard")
}

// OIDCTest reproduz POST /api/auth/oidc/test.
func (h *HTTP) OIDCTest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	settings, err := h.Settings(r.Context())
	if err != nil {
		reply(w, 500, map[string]string{"error": "OIDC test failed"})
		return
	}
	// Barreira própria: requireLogin=false ou sessão válida.
	if settings["requireLogin"] != false && h.session(r) == nil {
		reply(w, 401, map[string]string{"error": "Unauthorized"})
		return
	}
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body == nil {
		body = map[string]any{}
	}
	issuerURL := strings.TrimSpace(firstNonEmpty(stringOf(body["issuerUrl"]), text(settings, "oidcIssuerUrl")))
	clientID := strings.TrimSpace(firstNonEmpty(stringOf(body["clientId"]), text(settings, "oidcClientId")))
	scopes := strings.TrimSpace(firstNonEmpty(stringOf(body["scopes"]), text(settings, "oidcScopes")))
	if scopes == "" {
		scopes = oidcDefaultScopes
	}
	clientSecret := ""
	if _, has := body["clientSecret"]; has {
		clientSecret = strings.TrimSpace(stringOf(body["clientSecret"]))
	} else {
		clientSecret = strings.TrimSpace(text(settings, "oidcClientSecret"))
	}
	if issuerURL == "" {
		reply(w, 400, map[string]string{"error": "Issuer URL is required"})
		return
	}
	if clientID == "" {
		reply(w, 400, map[string]string{"error": "Client ID is required"})
		return
	}
	discovery, err := h.FetchOidcDiscovery(issuerURL)
	if err != nil {
		// O original devolve error.message; mantemos mensagem equivalente.
		reply(w, 500, map[string]string{"error": err.Error()})
		return
	}
	redirectURI := GetPublicOrigin(r) + "/api/auth/oidc/callback"
	tested, valid, message := h.ProbeOidcClientSecret(discovery.TokenEndpoint, clientID, clientSecret, redirectURI)
	if tested && valid == false {
		reply(w, 200, map[string]any{
			"ok":                    false,
			"discoveryOk":           true,
			"clientSecretTested":    true,
			"clientSecretValid":     false,
			"issuerUrl":             issuerURL,
			"clientId":              clientID,
			"scopes":                scopes,
			"redirectUri":           redirectURI,
			"authorizationEndpoint": discovery.AuthorizationEndpoint,
			"tokenEndpoint":         discovery.TokenEndpoint,
			"jwksUri":               discovery.JWKSURI,
			"error":                 "Discovery loaded, but the client secret is not valid: " + message,
		})
		return
	}
	reply(w, 200, map[string]any{
		"ok":                    true,
		"discoveryOk":           true,
		"clientSecretTested":    tested,
		"clientSecretValid":     valid,
		"issuerUrl":             issuerURL,
		"clientId":              clientID,
		"scopes":                scopes,
		"redirectUri":           redirectURI,
		"authorizationEndpoint": discovery.AuthorizationEndpoint,
		"tokenEndpoint":         discovery.TokenEndpoint,
		"jwksUri":               discovery.JWKSURI,
		"message":               message,
	})
}

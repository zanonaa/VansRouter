package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	oidcDefaultScopes   = "openid profile email"
	oidcDefaultLabel    = "Sign in with OIDC"
	oidcCookieMaxAgeSec = 10 * 60
)

// Discovery representa o documento .well-known/openid-configuration.
type Discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type OIDCRuntime struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Scopes       string
	LoginLabel   string
}

func trimTrailingSlashes(value string) string {
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

func normalizeScopes(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return oidcDefaultScopes
	}
	return trimmed
}

// GetPublicOrigin reproduz getPublicOrigin.
func GetPublicOrigin(r *http.Request) string {
	if v := baseURLFromEnv(); v != "" {
		return trimTrailingSlashes(v)
	}
	forwardedProto := r.Header.Get("X-Forwarded-Proto")
	host := firstNonEmpty(r.Header.Get("X-Forwarded-Host"), r.Header.Get("Host"))
	if host != "" {
		protocol := forwardedProto
		if protocol == "" {
			protocol = "http"
			if r.TLS != nil {
				protocol = "https"
			}
		}
		return strings.TrimRight(protocol+"://"+host, "/")
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return strings.TrimRight(scheme+"://"+r.Host, "/")
}

func baseURLFromEnv() string {
	return strings.TrimSpace(firstNonEmpty(os.Getenv("BASE_URL"), os.Getenv("NEXT_PUBLIC_BASE_URL")))
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// OidcRuntime reproduz getOidcRuntimeConfig sobre as settings mescladas.
func OidcRuntime(settings map[string]any) *OIDCRuntime {
	mode, _ := settings["authMode"].(string)
	if mode != "oidc" && mode != "both" {
		return nil
	}
	if !configured(settings) {
		return nil
	}
	issuerURL := trimTrailingSlashes(text(settings, "oidcIssuerUrl"))
	label := strings.TrimSpace(text(settings, "oidcLoginLabel"))
	if label == "" {
		label = oidcDefaultLabel
	}
	return &OIDCRuntime{
		IssuerURL:    issuerURL,
		ClientID:     strings.TrimSpace(text(settings, "oidcClientId")),
		ClientSecret: strings.TrimSpace(text(settings, "oidcClientSecret")),
		Scopes:       normalizeScopes(text(settings, "oidcScopes")),
		LoginLabel:   label,
	}
}

func (h *HTTP) oidcClient() *http.Client {
	if h.OIDCClient != nil {
		return h.OIDCClient
	}
	return defaultOIDCClient
}

var defaultOIDCClient = &http.Client{Timeout: 15 * time.Second}

// FetchOidcDiscovery reproduz fetchOidcDiscovery.
func (h *HTTP) FetchOidcDiscovery(issuerURL string) (*Discovery, error) {
	discoveryURL := trimTrailingSlashes(issuerURL) + "/.well-known/openid-configuration"
	res, err := h.oidcClient().Get(discoveryURL)
	if err != nil {
		return nil, fmt.Errorf("Failed to load OIDC discovery document from %s", discoveryURL)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Failed to load OIDC discovery document from %s", discoveryURL)
	}
	var doc Discovery
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, fmt.Errorf("Failed to load OIDC discovery document from %s", discoveryURL)
	}
	return &doc, nil
}

func base64urlDecode(value string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
}

func randomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// CreatePkcePair reproduz createPkcePair.
func CreatePkcePair() (verifier, challenge string, err error) {
	verifier, err = randomBase64URL(32)
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

// CreateOidcState/Nonce reproduzem os geradores de 16 bytes.
func CreateOidcState() (string, error) { return randomBase64URL(16) }
func CreateOidcNonce() (string, error) { return randomBase64URL(16) }

// BuildOidcAuthorizationURL reproduz buildOidcAuthorizationUrl.
func BuildOidcAuthorizationURL(authorizationEndpoint string, clientID, redirectURI, scopes, state, nonce, codeChallenge string) string {
	u, err := url.Parse(authorizationEndpoint)
	if err != nil {
		return authorizationEndpoint
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", normalizeScopes(scopes))
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String()
}

// ExchangeOidcCode reproduz exchangeOidcCode.
func (h *HTTP) ExchangeOidcCode(tokenEndpoint, clientID, clientSecret, code, redirectURI, codeVerifier string) (map[string]any, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", codeVerifier)
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	res, err := h.oidcClient().Post(tokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, errors.New("OIDC token exchange failed (network)")
	}
	defer res.Body.Close()
	var data map[string]any
	_ = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&data)
	if data == nil {
		data = map[string]any{}
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		message := fmt.Sprintf("OIDC token exchange failed (%d)", res.StatusCode)
		if d, _ := data["error_description"].(string); d != "" {
			message = d
		} else if e, _ := data["error"].(string); e != "" {
			message = e
		}
		return nil, errors.New(message)
	}
	return data, nil
}

// jwksKey representa uma chave do documento JWKS.
type jwksKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func parseJWK(k jwksKey) (any, error) {
	switch k.Kty {
	case "RSA":
		nBytes, err := base64urlDecode(k.N)
		if err != nil {
			return nil, errors.New("JWK RSA inválido")
		}
		eBytes, err := base64urlDecode(k.E)
		if err != nil || len(eBytes) > 8 {
			return nil, errors.New("JWK RSA inválido")
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 | int(b)
		}
		if e < 3 || e%2 == 0 {
			return nil, errors.New("JWK RSA inválido")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
	case "EC":
		var curve elliptic.Curve
		switch k.Crv {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, errors.New("JWK EC com curva não suportada")
		}
		xBytes, err := base64urlDecode(k.X)
		if err != nil {
			return nil, errors.New("JWK EC inválido")
		}
		yBytes, err := base64urlDecode(k.Y)
		if err != nil {
			return nil, errors.New("JWK EC inválido")
		}
		x := new(big.Int).SetBytes(xBytes)
		y := new(big.Int).SetBytes(yBytes)
		if !curve.IsOnCurve(x, y) {
			return nil, errors.New("JWK EC inválido")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	default:
		return nil, errors.New("JWK com kty não suportado")
	}
}

// fetchJWKS obtém e converte as chaves do JWKS.
func (h *HTTP) fetchJWKS(jwksURI string) ([]jwksKey, error) {
	res, err := h.oidcClient().Get(jwksURI)
	if err != nil {
		return nil, errors.New("failed to load JWKS")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, errors.New("failed to load JWKS")
	}
	var doc struct {
		Keys []jwksKey `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&doc); err != nil {
		return nil, errors.New("failed to load JWKS")
	}
	return doc.Keys, nil
}

var oidcAllowedAlgs = []string{"RS256", "RS384", "RS512", "PS256", "PS384", "PS512", "ES256", "ES384", "ES512"}

// VerifyOidcIdToken reproduz verifyOidcIdToken (jose createRemoteJWKSet + jwtVerify com issuer/audience/nonce).
func (h *HTTP) VerifyOidcIdToken(idToken, issuer, audience, jwksURI, nonce string) (jwt.MapClaims, error) {
	keys, err := h.fetchJWKS(jwksURI)
	if err != nil {
		return nil, err
	}
	// kid do header para seleção de chave.
	headerKid := ""
	if parts := strings.Split(idToken, "."); len(parts) == 3 {
		raw, err := base64urlDecode(parts[0])
		if err == nil {
			var header struct {
				Alg string `json:"alg"`
				Kid string `json:"kid"`
			}
			if json.Unmarshal(raw, &header) == nil {
				headerKid = header.Kid
			}
		}
	}
	candidates := []jwksKey{}
	for _, k := range keys {
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		if headerKid != "" && k.Kid != "" && k.Kid != headerKid {
			continue
		}
		candidates = append(candidates, k)
	}
	if len(candidates) == 0 {
		return nil, errors.New("no applicable JWKS key found")
	}
	var lastErr error = errors.New("no applicable JWKS key found")
	for _, k := range candidates {
		pub, err := parseJWK(k)
		if err != nil {
			lastErr = err
			continue
		}
		token, err := jwt.Parse(idToken, func(t *jwt.Token) (any, error) {
			alg, _ := t.Header["alg"].(string)
			// Confusão de algoritmo: família deve corresponder ao tipo da chave.
			switch alg {
			case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512":
				if _, ok := pub.(*rsa.PublicKey); !ok {
					return nil, errors.New("algoritmo incompatível com a chave")
				}
			case "ES256", "ES384", "ES512":
				if _, ok := pub.(*ecdsa.PublicKey); !ok {
					return nil, errors.New("algoritmo incompatível com a chave")
				}
			default:
				return nil, errors.New("algoritmo não suportado")
			}
			return pub, nil
		}, jwt.WithValidMethods(oidcAllowedAlgs), jwt.WithIssuer(issuer), jwt.WithAudience(audience), jwt.WithExpirationRequired(), jwt.WithLeeway(60*time.Second))
		if err != nil {
			lastErr = err
			continue
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || !token.Valid {
			lastErr = errors.New("id_token inválido")
			continue
		}
		if nonce != "" {
			got, _ := claims["nonce"].(string)
			if got != nonce {
				lastErr = errors.New("nonce mismatch")
				continue
			}
		}
		return claims, nil
	}
	return nil, lastErr
}

// PickOidcDisplayName/Email reproduzem os seletores originais.
func PickOidcDisplayName(payload jwt.MapClaims) string {
	for _, key := range []string{"preferred_username", "email", "name", "given_name", "sub"} {
		if v, _ := payload[key].(string); v != "" {
			return v
		}
	}
	return "OIDC user"
}

func PickOidcEmail(payload jwt.MapClaims) string {
	v, _ := payload["email"].(string)
	return v
}

// ProbeOidcClientSecret reproduz probeOidcClientSecret.
func (h *HTTP) ProbeOidcClientSecret(tokenEndpoint, clientID, clientSecret, redirectURI string) (tested bool, valid any, message string) {
	if clientSecret == "" {
		return false, nil, "No client secret was provided, so secret validation was skipped."
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", "__oidc_test_invalid_code__")
	form.Set("redirect_uri", redirectURI)
	form.Set("code_verifier", "__oidc_test_invalid_verifier__")
	res, err := h.oidcClient().Post(tokenEndpoint, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		return true, nil, "Token endpoint request failed"
	}
	defer res.Body.Close()
	var data map[string]any
	_ = json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&data)
	if data == nil {
		data = map[string]any{}
	}
	errName := strings.ToLower(stringOf(data["error"]))
	errDesc := firstNonEmpty(stringOf(data["error_description"]), stringOf(data["error"]))
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return true, true, "Client secret was accepted by the token endpoint."
	}
	if errName == "invalid_client" || errName == "unauthorized_client" || containsClientInvalid(errDesc) {
		return true, false, firstNonEmpty(errDesc, "Client secret is not valid.")
	}
	if errName == "invalid_grant" || errName == "invalid_code" || matchesGrantOrCode(errDesc) {
		return true, true, "Client secret was accepted; the token exchange failed only because the test authorization code is invalid."
	}
	return true, nil, firstNonEmpty(errDesc, fmt.Sprintf("Token endpoint responded with %d", res.StatusCode))
}

func stringOf(v any) string {
	s, _ := v.(string)
	return s
}

func containsClientInvalid(desc string) bool {
	lower := strings.ToLower(desc)
	return strings.Contains(lower, "client") && (strings.Contains(lower, "invalid") || strings.Contains(lower, "failed") || strings.Contains(lower, "mismatch"))
}

func matchesGrantOrCode(desc string) bool {
	lower := strings.ToLower(desc)
	return strings.Contains(lower, "grant") || strings.Contains(lower, "code")
}

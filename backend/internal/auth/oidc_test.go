package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// fakeIdP monta um provedor OIDC mínimo com assinatura RS256 real.
type fakeIdP struct {
	server     *httptest.Server
	key        *rsa.PrivateKey
	issuer     string
	nonce      string
	extraAud   string
	badNonce   bool
	tokenCalls int
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &fakeIdP{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"issuer":                 idp.issuer,
			"authorization_endpoint": idp.issuer + "/authorize",
			"token_endpoint":         idp.issuer + "/token",
			"jwks_uri":               idp.issuer + "/jwks",
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, r *http.Request) {
		pub := idp.key.PublicKey
		e := big.NewInt(int64(pub.E))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA", "kid": "test-key", "use": "sig", "alg": "RS256",
				"n": base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e": base64.RawURLEncoding.EncodeToString(e.Bytes()),
			}},
		})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		idp.tokenCalls++
		_ = r.ParseForm()
		if r.FormValue("client_secret") != "segredo-ficticio" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"client authentication failed"}`))
			return
		}
		nonce := idp.nonce
		if idp.badNonce {
			nonce = "nonce-errado"
		}
		claims := jwt.MapClaims{
			"iss":                idp.issuer,
			"sub":                "usuario-123",
			"aud":                "cliente-ficticio",
			"preferred_username": "Operador",
			"email":              "operador@example.test",
			"iat":                time.Now().Unix(),
			"exp":                time.Now().Add(time.Hour).Unix(),
		}
		if nonce != "" {
			claims["nonce"] = nonce
		}
		if idp.extraAud != "" {
			claims["aud"] = []string{"cliente-ficticio", idp.extraAud}
		}
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = "test-key"
		signed, err := token.SignedString(idp.key)
		if err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"id_token": signed, "access_token": "ficticio", "token_type": "Bearer"})
	})
	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)
	idp.issuer = idp.server.URL
	return idp
}

func oidcHTTP(t *testing.T, idp *fakeIdP) *HTTP {
	t.Helper()
	h := testHTTP(t)
	h.Settings = func(context.Context) (map[string]any, error) {
		return map[string]any{
			"authMode": "oidc", "oidcIssuerUrl": idp.issuer,
			"oidcClientId": "cliente-ficticio", "oidcClientSecret": "segredo-ficticio",
			"requireLogin": true,
		}, nil
	}
	h.OIDCClient = idp.server.Client()
	return h
}

func TestOIDCStartFlow(t *testing.T) {
	idp := newFakeIdP(t)
	h := oidcHTTP(t, idp)
	w := httptest.NewRecorder()
	r := request("GET", "/api/auth/oidc/start", "", "127.0.0.1:1234")
	h.OIDCStart(w, r)
	if w.Code != 307 {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	if !strings.HasPrefix(location, idp.issuer+"/authorize?") {
		t.Fatalf("location divergente: %s", location)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}
	q := parsed.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != "cliente-ficticio" ||
		q.Get("code_challenge_method") != "S256" || q.Get("redirect_uri") == "" ||
		q.Get("state") == "" || q.Get("nonce") == "" || q.Get("code_challenge") == "" ||
		q.Get("scope") != "openid profile email" {
		t.Fatalf("parâmetros divergentes: %v", q)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 3 {
		t.Fatalf("cookies: %v", cookies)
	}
	for _, c := range cookies {
		if !c.HttpOnly || c.Path != "/" || c.MaxAge != 600 || c.SameSite != http.SameSiteLaxMode {
			t.Fatalf("cookie divergente: %+v", c)
		}
	}
	// Mitigações CSRF/prefetch.
	for _, tt := range []struct{ header, value string }{
		{"Sec-Purpose", "prefetch"}, {"Purpose", "prefetch"},
		{"Sec-Fetch-Mode", "cors"}, {"Sec-Fetch-Dest", "script"},
	} {
		w = httptest.NewRecorder()
		r = request("GET", "/api/auth/oidc/start", "", "127.0.0.1:1234")
		r.Header.Set(tt.header, tt.value)
		h.OIDCStart(w, r)
		if w.Code != 403 {
			t.Fatalf("%s=%s deveria 403: %d", tt.header, tt.value, w.Code)
		}
	}
	// Sem configuração.
	w = httptest.NewRecorder()
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{}, nil }
	r = request("GET", "/api/auth/oidc/start", "", "127.0.0.1:1234")
	h.OIDCStart(w, r)
	if w.Code != 307 || !strings.Contains(w.Header().Get("Location"), "error=oidc_not_configured") {
		t.Fatalf("sem config: %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestOIDCCallbackFlow(t *testing.T) {
	idp := newFakeIdP(t)
	h := oidcHTTP(t, idp)
	// start para obter cookies legítimos
	w := httptest.NewRecorder()
	h.OIDCStart(w, request("GET", "/api/auth/oidc/start", "", "127.0.0.1:1234"))
	state, nonce, verifier := "", "", ""
	for _, c := range w.Result().Cookies() {
		switch c.Name {
		case "oidc_state":
			state = c.Value
		case "oidc_nonce":
			nonce = c.Value
		case "oidc_code_verifier":
			verifier = c.Value
		}
	}
	idp.nonce = nonce
	// callback válido
	r := request("GET", "/api/auth/oidc/callback?code=codigo-ficticio&state="+url.QueryEscape(state), "", "127.0.0.1:1234")
	for name, value := range map[string]string{"oidc_state": state, "oidc_nonce": nonce, "oidc_code_verifier": verifier} {
		r.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	w = httptest.NewRecorder()
	h.OIDCCallback(w, r)
	if w.Code != 307 || !strings.HasSuffix(w.Header().Get("Location"), "/dashboard") {
		t.Fatalf("callback: %d %s %s", w.Code, w.Header().Get("Location"), w.Body.String())
	}
	var session *http.Cookie
	cleared := 0
	for _, c := range w.Result().Cookies() {
		if c.Name == "auth_token" {
			session = c
		} else if c.MaxAge == -1 {
			cleared++
		}
	}
	if session == nil || cleared != 3 {
		t.Fatalf("cookies do callback divergentes: %+v", w.Result().Cookies())
	}
	claims, err := h.Sessions.Verify(session.Value)
	if err != nil || claims["oidc"] != true || claims["oidcName"] != "Operador" || claims["oidcEmail"] != "operador@example.test" || claims["oidcSub"] != "usuario-123" {
		t.Fatalf("claims OIDC divergentes: %v %v", claims, err)
	}
	// status agora reflete a sessão OIDC
	w = httptest.NewRecorder()
	r = request("GET", "/api/auth/status", "", "127.0.0.1:1234")
	r.AddCookie(session)
	h.Status(w, r)
	if !strings.Contains(w.Body.String(), `"displayName":"Operador"`) || !strings.Contains(w.Body.String(), `"loginMethod":"OIDC"`) {
		t.Fatalf("status OIDC: %s", w.Body.String())
	}
}

func TestOIDCCallbackFailures(t *testing.T) {
	idp := newFakeIdP(t)
	h := oidcHTTP(t, idp)
	// error do IdP
	w := httptest.NewRecorder()
	h.OIDCCallback(w, request("GET", "/api/auth/oidc/callback?error=access_denied", "", "127.0.0.1:1234"))
	if w.Code != 307 || !strings.Contains(w.Header().Get("Location"), "error=access_denied") {
		t.Fatalf("error do IdP: %d", w.Code)
	}
	// sem code/state
	w = httptest.NewRecorder()
	h.OIDCCallback(w, request("GET", "/api/auth/oidc/callback", "", "127.0.0.1:1234"))
	if !strings.Contains(w.Header().Get("Location"), "oidc_missing_code") {
		t.Fatalf("sem code: %s", w.Header().Get("Location"))
	}
	// state divergente
	r := request("GET", "/api/auth/oidc/callback?code=x&state=falsificado", "", "127.0.0.1:1234")
	r.AddCookie(&http.Cookie{Name: "oidc_state", Value: "legitimo"})
	r.AddCookie(&http.Cookie{Name: "oidc_nonce", Value: "n"})
	r.AddCookie(&http.Cookie{Name: "oidc_code_verifier", Value: "v"})
	w = httptest.NewRecorder()
	h.OIDCCallback(w, r)
	if !strings.Contains(w.Header().Get("Location"), "oidc_invalid_state") {
		t.Fatalf("state falsificado: %s", w.Header().Get("Location"))
	}
	// nonce divergente: token emitido com nonce errado deve falhar na verificação
	w = httptest.NewRecorder()
	h.OIDCStart(w, request("GET", "/api/auth/oidc/start", "", "127.0.0.1:1234"))
	state, nonce, verifier := "", "", ""
	for _, c := range w.Result().Cookies() {
		switch c.Name {
		case "oidc_state":
			state = c.Value
		case "oidc_nonce":
			nonce = c.Value
		case "oidc_code_verifier":
			verifier = c.Value
		}
	}
	idp.nonce = nonce
	idp.badNonce = true
	r = request("GET", "/api/auth/oidc/callback?code=codigo&state="+url.QueryEscape(state), "", "127.0.0.1:1234")
	for name, value := range map[string]string{"oidc_state": state, "oidc_nonce": nonce, "oidc_code_verifier": verifier} {
		r.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	w = httptest.NewRecorder()
	h.OIDCCallback(w, r)
	if w.Code == 307 && strings.HasSuffix(w.Header().Get("Location"), "/dashboard") {
		t.Fatal("callback aceitou nonce divergente")
	}
	// secret errado → troca falha
	idp.badNonce = false
	h.Settings = func(context.Context) (map[string]any, error) {
		return map[string]any{"authMode": "oidc", "oidcIssuerUrl": idp.issuer, "oidcClientId": "cliente-ficticio", "oidcClientSecret": "errado"}, nil
	}
	r = request("GET", "/api/auth/oidc/callback?code=codigo&state="+url.QueryEscape(state), "", "127.0.0.1:1234")
	for name, value := range map[string]string{"oidc_state": state, "oidc_nonce": nonce, "oidc_code_verifier": verifier} {
		r.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	w = httptest.NewRecorder()
	h.OIDCCallback(w, r)
	if !strings.Contains(w.Header().Get("Location"), "error=") {
		t.Fatal("secret errado não retornou erro")
	}
}

func TestOIDCTestRoute(t *testing.T) {
	idp := newFakeIdP(t)
	h := oidcHTTP(t, idp)
	// sem sessão e requireLogin=true → 401
	w := httptest.NewRecorder()
	h.OIDCTest(w, request("POST", "/api/auth/oidc/test", "{}", "127.0.0.1:1234"))
	if w.Code != 401 {
		t.Fatalf("barreira: %d", w.Code)
	}
	// com sessão
	session, _ := h.Sessions.Create(nil)
	r := request("POST", "/api/auth/oidc/test", `{}`, "127.0.0.1:1234")
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: session})
	w = httptest.NewRecorder()
	h.OIDCTest(w, r)
	if w.Code != 200 {
		t.Fatalf("test: %d %s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["ok"] != true || body["discoveryOk"] != true || body["clientSecretTested"] != true || body["clientSecretValid"] != true {
		t.Fatalf("probe divergente: %v", body)
	}
	if body["issuerUrl"] != idp.issuer || body["authorizationEndpoint"] != idp.issuer+"/authorize" || body["tokenEndpoint"] != idp.issuer+"/token" || body["jwksUri"] != idp.issuer+"/jwks" {
		t.Fatalf("campos do discovery divergentes: %v", body)
	}
	if !strings.Contains(body["message"].(string), "accepted") {
		t.Fatalf("mensagem divergente: %v", body["message"])
	}
	// secret inválido → ok:false
	r = request("POST", "/api/auth/oidc/test", `{"clientSecret":"errado"}`, "127.0.0.1:1234")
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: session})
	w = httptest.NewRecorder()
	h.OIDCTest(w, r)
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body["ok"] != false || body["clientSecretValid"] != false {
		t.Fatalf("secret inválido divergente: %v", body)
	}
	// issuer ausente (corpo e settings) → 400
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{}, nil }
	r = request("POST", "/api/auth/oidc/test", `{}`, "127.0.0.1:1234")
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: session})
	w = httptest.NewRecorder()
	h.OIDCTest(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Issuer URL is required") {
		t.Fatalf("issuer ausente: %d %s", w.Code, w.Body.String())
	}
	// clientId ausente → 400
	r = request("POST", "/api/auth/oidc/test", `{"issuerUrl":"https://x"}`, "127.0.0.1:1234")
	r.AddCookie(&http.Cookie{Name: "auth_token", Value: session})
	w = httptest.NewRecorder()
	h.OIDCTest(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Client ID is required") {
		t.Fatalf("clientId ausente: %d %s", w.Code, w.Body.String())
	}
}

func TestVerifyOidcIdTokenAlgConfusion(t *testing.T) {
	idp := newFakeIdP(t)
	h := oidcHTTP(t, idp)
	// Token HS256 assinado com o segredo JWT do servidor não deve ser aceito.
	claims := jwt.MapClaims{"iss": idp.issuer, "aud": "cliente-ficticio", "exp": time.Now().Add(time.Hour).Unix(), "nonce": "n", "sub": "x"}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	raw, err := token.SignedString([]byte("segredo-ficticio-para-testes"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.VerifyOidcIdToken(raw, idp.issuer, "cliente-ficticio", idp.issuer+"/jwks", "n"); err == nil {
		t.Fatal("token HS256 aceito no fluxo OIDC")
	}
}

func TestGetPublicOrigin(t *testing.T) {
	r := request("GET", "/api/auth/oidc/start", "", "127.0.0.1:1234")
	r.Host = "dash.exemplo:20128"
	if got := GetPublicOrigin(r); got != "http://dash.exemplo:20128" {
		t.Fatalf("origin: %q", got)
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-Host", "tunel.exemplo")
	if got := GetPublicOrigin(r); got != "https://tunel.exemplo" {
		t.Fatalf("origin com proxy: %q", got)
	}
	t.Setenv("BASE_URL", "https://base.exemplo/")
	if got := GetPublicOrigin(r); got != "https://base.exemplo" {
		t.Fatalf("origin com BASE_URL: %q", got)
	}
}

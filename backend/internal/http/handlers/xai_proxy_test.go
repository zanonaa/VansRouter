package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// setXaiDiscoveryOverride fixa o discovery p/ os endpoints do servidor de
// testes, evitando rede externa.
func setXaiDiscoveryOverride(t *testing.T, endpoints xaiEndpoints) {
	t.Helper()
	previous := xaiDiscoveryCache
	xaiDiscoveryCache = &endpoints
	t.Cleanup(func() { xaiDiscoveryCache = previous })
}

func xaiEngine(t *testing.T, client *http.Client) (*OAuthEngine, *map[string]any) {
	t.Helper()
	saved := map[string]any{}
	store := &saved
	engine := &OAuthEngine{Client: client, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		*store = data
		data["id"] = "test-id"
		return data, nil
	}}
	return engine, store
}

func withChiParams(t *testing.T, r *http.Request, values map[string]string) *http.Request {
	t.Helper()
	route := chi.NewRouteContext()
	for key, value := range values {
		route.URLParams.Add(key, value)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, route))
}

// buildFakeJWT cria um JWT sem assinatura com os claims dados.
func buildFakeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join([]string{
		base64.RawURLEncoding.EncodeToString(header),
		base64.RawURLEncoding.EncodeToString(payload),
		"c2lnbmF0dXJl",
	}, ".")
}

func routeGet(t *testing.T, engine *OAuthEngine, provider, action, rawQuery string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/oauth/%s/%s?%s", provider, action, rawQuery), nil)
	req = withChiParams(t, req, map[string]string{"provider": provider, "action": action})
	w := httptest.NewRecorder()
	engine.Route(w, req)
	return w
}

func routePost(t *testing.T, engine *OAuthEngine, provider, action, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/oauth/%s/%s", provider, action), strings.NewReader(body))
	req = withChiParams(t, req, map[string]string{"provider": provider, "action": action})
	w := httptest.NewRecorder()
	engine.Route(w, req)
	return w
}

func TestXaiAuthorizeContract(t *testing.T) {
	var serverURL string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"authorization_endpoint":"` + serverURL + `/authorize","token_endpoint":"` + serverURL + `/token"}`))
	}))
	defer upstream.Close()
	serverURL = upstream.URL
	setXaiDiscoveryOverride(t, xaiEndpoints{AuthorizeURL: upstream.URL + "/authorize", TokenURL: upstream.URL + "/token"})
	engine, _ := xaiEngine(t, upstream.Client())
	w := routeGet(t, engine, "xai", "authorize", "") // redirect_uri padrão aplicado dentro
	if w.Code != 200 {
		t.Fatalf("authorize %d %s", w.Code, w.Body.String())
	}
	var auth struct {
		AuthURL     string `json:"authUrl"`
		State       string `json:"state"`
		Verifier    string `json:"codeVerifier"`
		Challenge   string `json:"codeChallenge"`
		RedirectURI string `json:"redirectUri"`
		FlowType    string `json:"flowType"`
		FixedPort   int    `json:"fixedPort"`
		Callback    string `json:"callbackPath"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &auth); err != nil {
		t.Fatal(err)
	}
	if auth.RedirectURI != "http://localhost:8080/callback" {
		t.Errorf("default redirect_uri %q", auth.RedirectURI)
	}
	params, err := url.Parse(auth.AuthURL)
	if err != nil || params.Host != upstream.Listener.Addr().String() {
		t.Fatalf("auth url %s", auth.AuthURL)
	}
	query := params.Query()
	sum := sha256.Sum256([]byte(auth.Verifier))
	if query.Get("response_type") != "code" || query.Get("client_id") != "b1a00492-073a-47ea-816f-4c329264a828" {
		t.Errorf("params %v", params)
	}
	if query.Get("scope") != "openid profile email offline_access grok-cli:access api:access" {
		t.Errorf("scope %q", query.Get("scope"))
	}
	if query.Get("code_challenge_method") != "S256" || query.Get("code_challenge") != base64.RawURLEncoding.EncodeToString(sum[:]) || query.Get("code_challenge") != auth.Challenge {
		t.Errorf("pkce %s / %s", query.Get("code_challenge"), auth.Challenge)
	}
	if query.Get("plan") != "generic" || query.Get("referrer") != "cli-proxy-api" || len(query.Get("nonce")) != 32 {
		t.Errorf("plan %q referrer %q nonce %d", query.Get("plan"), query.Get("referrer"), len(query.Get("nonce")))
	}
	if len(auth.Verifier) != 128 || len(auth.State) != 43 {
		t.Errorf("verifier %d state %d", len(auth.Verifier), len(auth.State))
	}
	if auth.FlowType != "authorization_code_pkce" || auth.FixedPort != 56121 || auth.Callback != "/callback" {
		t.Errorf("meta %+v", auth)
	}
}

func TestXaiAuthorizeDiscoveryFallback(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"authorization_endpoint":"http://evil.example/authorize","token_endpoint":"https://auth.x.ai/oauth2/token"}`))
	}))
	defer upstream.Close()
	setXaiDiscoveryOverride(t, xaiEndpoints{})
	xaiDiscoveryCache = nil
	engine, _ := xaiEngine(t, upstream.Client())
	w := routeGet(t, engine, "xai", "authorize", "redirect_uri=http://127.0.0.1:56121/callback")
	if w.Code != 200 || !strings.Contains(w.Body.String(), "https://auth.x.ai/oauth2/authorize") {
		t.Fatalf("fallback not used %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "evil.example") {
		t.Fatalf("off-domain discovery accepted %s", w.Body.String())
	}
}

func TestXaiExchangeMapsIDToken(t *testing.T) {
	idToken := buildFakeJWT(t, map[string]any{"email": "dev@xai.example", "sub": "sub-1"})
	var formBody string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		formBody = r.Form.Encode()
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("content type %q", r.Header.Get("Content-Type"))
		}
		_, _ = w.Write([]byte(`{"access_token":"grok-access","refresh_token":"grok-refresh","expires_in":3600,"scope":"openid profile","id_token":"` + idToken + `"}`))
	}))
	defer upstream.Close()
	engine, saved := xaiEngine(t, upstream.Client())
	setXaiDiscoveryOverride(t, xaiEndpoints{AuthorizeURL: upstream.URL + "/authorize", TokenURL: upstream.URL + "/token"})
	payload, _ := json.Marshal(map[string]string{
		"code": "xai-code", "redirectUri": "http://127.0.0.1:56121/callback", "codeVerifier": strings.Repeat("v", 128),
	})
	w := routePost(t, engine, "xai", "exchange", string(payload))
	if w.Code != 200 {
		t.Fatalf("exchange %d %s", w.Code, w.Body.String())
	}
	for _, expected := range []string{"grant_type=authorization_code", "client_id=b1a00492-073a-47ea-816f-4c329264a828", "code=xai-code", "code_verifier=" + strings.Repeat("v", 128)} {
		if !strings.Contains(formBody, expected) {
			t.Errorf("missing %q in %q", expected, formBody)
		}
	}
	if strings.Contains(formBody, "client_secret") {
		t.Error("xai é cliente público: sem client_secret")
	}
	if (*saved)["provider"] != "xai" || (*saved)["authType"] != "oauth" || (*saved)["testStatus"] != "active" {
		t.Fatalf("connection meta %v", *saved)
	}
	if (*saved)["accessToken"] != "grok-access" || (*saved)["refreshToken"] != "grok-refresh" || (*saved)["scope"] != "openid profile" {
		t.Errorf("tokens %v", *saved)
	}
	if (*saved)["email"] != "dev@xai.example" {
		t.Errorf("email %v", (*saved)["email"])
	}
	specific, ok := (*saved)["providerSpecificData"].(map[string]any)
	if !ok || specific["idToken"] != idToken {
		t.Fatalf("specific %v", (*saved)["providerSpecificData"])
	}
	if (*saved)["expiresAt"] == nil || (*saved)["expiresIn"] != float64(3600) {
		t.Errorf("expiry %v", *saved)
	}
	if _, leaked := (*saved)["idToken"]; leaked {
		t.Error("id_token cru não deve ser salvo no topo")
	}
	if strings.Contains(w.Body.String(), "grok-access") {
		t.Fatal("token vazou na resposta")
	}
}

func TestXaiManualCodeContract(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"grok-access","refresh_token":"grok-refresh","expires_in":3600,"id_token":"` + buildFakeJWT(t, map[string]any{"email": "dev@xai.example"}) + `"}`))
	}))
	defer upstream.Close()
	engine, saved := xaiEngine(t, upstream.Client())
	setXaiDiscoveryOverride(t, xaiEndpoints{AuthorizeURL: upstream.URL + "/authorize", TokenURL: upstream.URL + "/token"})

	w := routePost(t, engine, "xai", "manual-code", `{"code":"code","state":""}`)
	if w.Code != 500 || !strings.Contains(w.Body.String(), "xAI OAuth session not found; restart the login flow and paste the code again") {
		t.Fatalf("no session %d %s", w.Code, w.Body.String())
	}
	if !registerProxySession("xai", "st1", "verifier-1", "http://127.0.0.1:56121/callback") {
		t.Fatal("register session")
	}
	if w := routePost(t, engine, "xai", "manual-code", `{"state":"st1"}`); w.Code != 500 || !strings.Contains(w.Body.String(), "Missing xAI authorization code") {
		t.Fatalf("missing code %d %s", w.Code, w.Body.String())
	}
	if getProxySession("xai", "st1") != nil {
		t.Fatal("sessão deve ser limpa após código ausente")
	}
	if !registerProxySession("xai", "st1", "verifier-1", "http://127.0.0.1:56121/callback") {
		t.Fatal("register session")
	}
	w = routePost(t, engine, "xai", "manual-code", `{"code":" xai-code ","state":" st1 "}`)
	if w.Code != 200 {
		t.Fatalf("manual code %d %s", w.Code, w.Body.String())
	}
	if (*saved)["provider"] != "xai" || (*saved)["accessToken"] != "grok-access" || (*saved)["email"] != "dev@xai.example" {
		t.Fatalf("saved %v", *saved)
	}
	if getProxySession("xai", "st1") != nil {
		t.Fatal("sessão deve ser limpa após sucesso")
	}
	var response struct {
		Success    bool           `json:"success"`
		Connection map[string]any `json:"connection"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	if !response.Success || response.Connection["id"] != "test-id" || response.Connection["provider"] != "xai" || response.Connection["email"] != "dev@xai.example" {
		t.Fatalf("response %s", w.Body.String())
	}
	// Guardas de provider e corpo inválido.
	w = routePost(t, engine, "codex", "manual-code", `{"code":"x","state":"y"}`)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Manual code only supported for xai") {
		t.Fatalf("provider guard %d %s", w.Code, w.Body.String())
	}
	w = routePost(t, engine, "xai", "manual-code", "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "Invalid or empty request body") {
		t.Fatalf("body guard %d %s", w.Code, w.Body.String())
	}
}

func TestXaiProxyLifecycle(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"proxy-access","refresh_token":"proxy-refresh","expires_in":3600,"id_token":"` + buildFakeJWT(t, map[string]any{"preferred_username": "grok.user"}) + `"}`))
	}))
	defer upstream.Close()
	engine, saved := xaiEngine(t, upstream.Client())
	setXaiDiscoveryOverride(t, xaiEndpoints{AuthorizeURL: upstream.URL + "/authorize", TokenURL: upstream.URL + "/token"})
	defer func() { stopCallbackProxy("xai") }()

	if w := routeGet(t, engine, "claude", "start-proxy", "app_port=9"); w.Code != 400 || !strings.Contains(w.Body.String(), "Proxy only supported for codex/xai") {
		t.Fatalf("guard %d %s", w.Code, w.Body.String())
	}
	if w := routeGet(t, engine, "claude", "poll-status", "state=st"); w.Code != 400 || !strings.Contains(w.Body.String(), "Poll only supported for codex/xai") {
		t.Fatalf("poll guard %d %s", w.Code, w.Body.String())
	}
	if w := routeGet(t, engine, "xai", "start-proxy", ""); w.Code != 400 || !strings.Contains(w.Body.String(), "Missing app_port") {
		t.Fatalf("app port %d %s", w.Code, w.Body.String())
	}
	if w := routeGet(t, engine, "xai", "start-proxy", "app_port=9&state=st2&code_verifier=cv&redirect_uri=http%3A%2F%2F127.0.0.1%3A56121%2Fcallback"); w.Code != 200 || !strings.Contains(w.Body.String(), `"success":true`) || !strings.Contains(w.Body.String(), `"serverSide":true`) {
		t.Fatalf("start %d %s", w.Code, w.Body.String())
	}
	// 404 fora de /callback e /auth/callback.
	res, err := http.Get("http://127.0.0.1:56121/other")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := readBody(res)
	if res.StatusCode != 404 || strings.TrimSpace(body) != "Not found" {
		t.Fatalf("404 esperado: %d %s", res.StatusCode, body)
	}
	// Origin cross-site é bloqueado (guard anti login-CSRF).
	req, _ := http.NewRequest("GET", "http://127.0.0.1:56121/callback?code=x&state=st2", nil)
	req.Header.Set("Origin", "https://evil.example")
	res, err = upstream.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ = readBody(res)
	if res.StatusCode != 403 || !strings.Contains(body, "Cross-origin callback rejected") {
		t.Fatalf("origin guard %d %s", res.StatusCode, body)
	}
	// Mode A: sessão registrada troca o código no servidor e salva a conexão.
	res, err = http.Get("http://127.0.0.1:56121/auth/callback?code=proxy-code&state=st2")
	if err != nil {
		t.Fatal(err)
	}
	body, _ = readBody(res)
	if res.StatusCode != 200 || !strings.Contains(body, "Authentication Successful") || res.Header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("mode A %d %s", res.StatusCode, body)
	}
	if (*saved)["provider"] != "xai" || (*saved)["accessToken"] != "proxy-access" || (*saved)["email"] != "grok.user" {
		t.Fatalf("proxy connection %v", *saved)
	}
	w := routeGet(t, engine, "xai", "poll-status", "state=st2")
	var status map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &status)
	if w.Code != 200 || status["status"] != "done" || status["connectionId"] != "test-id" || status["email"] != "grok.user" || status["codeVerifier"] != "cv" || status["redirectUri"] != "http://127.0.0.1:56121/callback" {
		t.Fatalf("poll status %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "proxy-access") {
		t.Fatal("token vazou via poll-status")
	}
	if w := routeGet(t, engine, "xai", "poll-status", "state=st2"); !strings.Contains(w.Body.String(), `"status":"unknown"`) {
		t.Fatalf("consumed status %d %s", w.Code, w.Body.String())
	}
	// Proxy encerrado após o consumo (finally do mode A): reiniciar escuta.
	if w := routeGet(t, engine, "xai", "start-proxy", "app_port=9"); w.Code != 200 || !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("restart %d %s", w.Code, w.Body.String())
	}
	// Mode B: sem sessão, redirect 302 para a porta do app com query original.
	redirect := httptest.NewRequest("GET", "http://127.0.0.1:56121/callback?code=y&state=other", nil)
	wRedirect := httptest.NewRecorder()
	engine.buildProxyHandler("xai", 9)(wRedirect, redirect)
	if wRedirect.Code != 302 || wRedirect.Header().Get("Location") != "http://localhost:9/callback?code=y&state=other" {
		t.Fatalf("mode B %d %s", wRedirect.Code, wRedirect.Header().Get("Location"))
	}
	// stop-proxy retorna success true.
	if w := routeGet(t, engine, "xai", "stop-proxy", ""); !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("stop %d %s", w.Code, w.Body.String())
	}
	if w := routeGet(t, engine, "xai", "start-proxy", "app_port=9"); !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("restart after stop %d %s", w.Code, w.Body.String())
	}
	stopCallbackProxy("xai")
	clearProxySession("xai", "st2")
}

func readBody(res *http.Response) (string, error) {
	defer res.Body.Close()
	buf := make([]byte, 8192)
	n, _ := res.Body.Read(buf)
	return string(buf[:n]), nil
}

func TestCodexProxyModeA(t *testing.T) {
	idToken := buildFakeJWT(t, map[string]any{"email": "dev@openai.example"})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"cx-access","refresh_token":"cx-refresh","expires_in":3600,"id_token":"` + idToken + `"}`))
	}))
	defer upstream.Close()
	engine, saved := xaiEngine(t, upstream.Client())
	codexConfig := *providers.GetOAuthConfig("codex")
	codexConfig.TokenURL = upstream.URL + "/token"
	engine.Config = func(string) *providers.OAuthConfig { return &codexConfig }
	defer func() { stopCallbackProxy("codex") }()

	w := routeGet(t, engine, "codex", "start-proxy", "app_port=10&state=st3&code_verifier=cv3&redirect_uri=http%3A%2F%2Flocalhost%3A1455%2Fcallback")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"serverSide":true`) {
		t.Fatalf("start codex %d %s", w.Code, w.Body.String())
	}
	res, err := http.Get("http://127.0.0.1:1455/callback?code=cx-code&state=st3")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := readBody(res)
	if res.StatusCode != 200 || !strings.Contains(body, "Authentication Successful") {
		t.Fatalf("codex mode A %d %s", res.StatusCode, body)
	}
	if (*saved)["provider"] != "codex" || (*saved)["authType"] != "oauth" || (*saved)["accessToken"] != "cx-access" || (*saved)["email"] != "dev@openai.example" {
		t.Fatalf("codex connection %v", *saved)
	}
	if _, exists := (*saved)["lastRefreshAt"]; !exists {
		t.Error("lastRefreshAt requerido (mapTokens codex)")
	}
	w = routeGet(t, engine, "codex", "poll-status", "state=st3")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"done"`) || !strings.Contains(w.Body.String(), `"email":"dev@openai.example"`) {
		t.Fatalf("codex poll %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "cx-access") {
		t.Fatal("token vazou via poll-status")
	}
	stopCallbackProxy("codex")
	clearProxySession("codex", "st3")
}

func TestDecodeXaiIDTokenEmailOrder(t *testing.T) {
	if email := decodeXaiIDTokenEmail(buildFakeJWT(t, map[string]any{"preferred_username": "user@x", "sub": "subs"})); email != "user@x" {
		t.Fatal(email)
	}
	if email := decodeXaiIDTokenEmail(buildFakeJWT(t, map[string]any{"sub": "subs"})); email != "subs" {
		t.Fatal(email)
	}
	if email := decodeXaiIDTokenEmail("not-a-jwt"); email != "" {
		t.Fatal(email)
	}
}

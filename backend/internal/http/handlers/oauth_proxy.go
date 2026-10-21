package handlers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Portas espelhando mode A/B do utils/server.js: com sessão registrada o proxy
// troca o código no servidor e salva a conexão; sem sessão, redireciona para o app.
const (
	xaiProxyPort      = 56121
	codexProxyPort    = 1455
	oauthProxyTimeout = 5 * time.Minute
)

var loopbackOriginPattern = regexp.MustCompile(`^http://(127\.0\.0\.1|localhost)(:\d+)?$`)

type oauthProxySession struct {
	CodeVerifier string      `json:"codeVerifier"`
	RedirectURI  string      `json:"redirectUri"`
	Status       string      `json:"status"`
	CreatedAt    int64       `json:"createdAt"`
	ConnectionID interface{} `json:"connectionId,omitempty"`
	Email        interface{} `json:"email,omitempty"`
	Error        string      `json:"error,omitempty"`
}

type callbackProxy struct {
	listener net.Listener
	server   *http.Server
	timer    *time.Timer
}

var (
	proxyMu       sync.Mutex
	xaiProxy      *callbackProxy
	codexProxy    *callbackProxy
	xaiSessions   = map[string]*oauthProxySession{}
	codexSessions = map[string]*oauthProxySession{}
)

func registerProxySession(provider, state, codeVerifier, redirectURI string) bool {
	if state == "" || codeVerifier == "" || redirectURI == "" {
		return false
	}
	proxyMu.Lock()
	defer proxyMu.Unlock()
	proxySessionMap(provider)[state] = &oauthProxySession{
		CodeVerifier: codeVerifier,
		RedirectURI:  redirectURI,
		Status:       "pending",
		CreatedAt:    time.Now().UnixMilli(),
	}
	return true
}

func proxySessionMap(provider string) map[string]*oauthProxySession {
	if provider == "xai" {
		return xaiSessions
	}
	return codexSessions
}

func getProxySession(provider, state string) *oauthProxySession {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	if state == "" {
		return nil
	}
	return proxySessionMap(provider)[state]
}

func clearProxySession(provider, state string) {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	delete(proxySessionMap(provider), state)
}

func isLoopbackOrigin(origin string) bool {
	if origin == "" {
		return true // redirect top-level legítimo — sem Origin
	}
	return loopbackOriginPattern.MatchString(origin)
}

func escapeHTMLText(value string) string {
	return strings.NewReplacer(
		"&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;",
	).Replace(value)
}

// renderResultPage espelha renderCodexResultPage do utils/server.js.
func renderResultPage(success bool, message string) string {
	color, icon, title := "#22c55e", "&#10003;", "Authentication Successful"
	if !success {
		color, icon, title = "#ef4444", "&#10007;", "Authentication Failed"
	}
	safeMessage := escapeHTMLText(message)
	return fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="utf-8"><title>%s</title>
<style>body{font-family:system-ui;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;background:#f5f5f5}.c{text-align:center;padding:2rem;background:#fff;border-radius:8px;box-shadow:0 2px 10px rgba(0,0,0,.1)}.i{color:%s;font-size:3rem}h1{margin:1rem 0}p{color:#666}</style>
</head><body><div class="c"><div class="i">%s</div><h1>%s</h1><p>%s</p><p>Closing in <span id="cd">3</span>s...</p>
<script>let n=3;const c=document.getElementById("cd");const t=setInterval(()=>{n--;c.textContent=n;if(n<=0){clearInterval(t);window.close();}},1000);</script>
</div></body></html>`, title, color, icon, title, safeMessage)
}

func proxyFor(provider string) *callbackProxy {
	if provider == "xai" {
		return xaiProxy
	}
	return codexProxy
}

func setProxyFor(provider string, state *callbackProxy) {
	if provider == "xai" {
		xaiProxy = state
		return
	}
	codexProxy = state
}

func startCallbackProxy(provider string, port, appPort int, handlerFunc func(http.ResponseWriter, *http.Request)) map[string]any {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	if proxyFor(provider) != nil {
		return map[string]any{"success": true}
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "address already in use") {
			return map[string]any{"success": false, "reason": "port_busy"}
		}
		return map[string]any{"success": false, "reason": err.Error()}
	}
	handler := handlerFunc
	server := &http.Server{Handler: http.HandlerFunc(handler)}
	state := &callbackProxy{listener: listener, server: server}
	setProxyFor(provider, state)
	state.timer = time.AfterFunc(oauthProxyTimeout, func() { stopCallbackProxy(provider) })
	go func() { _ = server.Serve(listener) }()
	return map[string]any{"success": true}
}

func stopCallbackProxy(provider string) {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	state := proxyFor(provider)
	if state == nil {
		return
	}
	setProxyFor(provider, nil)
	if state.timer != nil {
		state.timer.Stop()
		state.timer = nil
	}
	// server.close() do Node: para de aceitar e libera a porta, sem cortar
	// conexões em voo (a resposta do callback precisa chegar ao cliente).
	_ = state.listener.Close()
}

// buildProxyHandler um único handler com guard de Origin e modos A/B.
func (h *OAuthEngine) buildProxyHandler(provider string, appPort int) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/callback" && r.URL.Path != "/auth/callback" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("Not found"))
			return
		}
		if !isLoopbackOrigin(r.Header.Get("Origin")) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(renderResultPage(false, "Cross-origin callback rejected")))
			return
		}
		query := r.URL.Query()
		code := query.Get("code")
		stateParam := query.Get("state")
		errorParam := query.Get("error")
		session := getProxySession(provider, stateParam)

		if session != nil {
			status, message := h.completeProxyExchange(r, provider, session, code, errorParam, query.Get("error_description"))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(renderResultPage(status, message)))
			stopCallbackProxy(provider)
			return
		}

		// Modo B: fallback do canal legado — 302 para a porta do app.
		target := fmt.Sprintf("http://localhost:%d/callback", appPort)
		if rawQuery := r.URL.RawQuery; rawQuery != "" {
			target += "?" + rawQuery
		}
		http.Redirect(w, r, target, http.StatusFound)
		stopCallbackProxy(provider)
	}
}

func (h *OAuthEngine) completeProxyExchange(r *http.Request, provider string, session *oauthProxySession, code, errorParam, errorDescription string) (bool, string) {
	var exchangeErr error
	switch {
	case errorParam != "":
		if errorDescription != "" {
			exchangeErr = errors.New(errorDescription)
		} else {
			exchangeErr = errors.New(errorParam)
		}
	case code == "":
		exchangeErr = errors.New("No authorization code received")
	default:
		exchangeErr = h.exchangeForProxy(provider, session, code)
	}
	if exchangeErr != nil {
		session.Status = "error"
		session.Error = exchangeErr.Error()
		return false, session.Error
	}
	session.Status = "done"
	return true, "You can close this window."
}

func (h *OAuthEngine) exchangeForProxy(provider string, session *oauthProxySession, code string) error {
	if h.CreateConnection == nil {
		return errors.New("Failed to save connection")
	}
	ctx := context.Background()
	var connectionData map[string]any
	if provider == "xai" {
		tokenData, err := h.exchangeXaiTokens(ctx, code, session.RedirectURI, session.CodeVerifier)
		if err != nil {
			return err
		}
		connectionData = xaiConnectionData(tokenData)
	} else {
		tokenData, err := h.exchangeTokens(ctx, provider, code, session.RedirectURI, session.CodeVerifier, "")
		if err != nil {
			return err
		}
		connectionData = codexConnectionData(tokenData)
	}
	connection, err := h.CreateConnection(ctx, connectionData)
	if err != nil {
		return errors.New("Failed to save connection")
	}
	session.ConnectionID = connection["id"]
	if email, ok := connection["email"]; ok {
		session.Email = email
	}
	return nil
}

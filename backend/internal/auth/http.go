package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type SettingsReader func(context.Context) (map[string]any, error)
type HTTP struct {
	Sessions     *Sessions
	Settings     SettingsReader
	Limiter      *Limiter
	Initial      string
	InitialValid bool
	ForceSecure  bool
	// CheckConfigured reproduz a exigência JWT_SECRET exclusiva de auth/check.
	CheckConfigured bool
}

const resetHint = "Forgot password? Use the VansRouter CLI on the host to set a new dashboard password."

func reply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func text(settings map[string]any, key string) string {
	value, _ := settings[key].(string)
	return value
}
func configured(settings map[string]any) bool {
	return strings.TrimRight(strings.TrimSpace(text(settings, "oidcIssuerUrl")), "/") != "" && strings.TrimSpace(text(settings, "oidcClientId")) != "" && strings.TrimSpace(text(settings, "oidcClientSecret")) != ""
}
func peer(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	return host
}
func local(r *http.Request) bool {
	ip := net.ParseIP(peer(r))
	if ip == nil || !ip.IsLoopback() {
		return false
	}
	// Um hop de proxy não equivale ao operador local. Headers não concedem confiança.
	if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Real-IP") != "" || r.Header.Get("X-9r-via-proxy") != "" {
		return false
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		host := u.Hostname()
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return false
		}
	}
	return true
}
func (h *HTTP) session(r *http.Request) map[string]any {
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return nil
	}
	claims, err := h.Sessions.Verify(cookie.Value)
	if err != nil {
		return nil
	}
	return claims
}
func (h *HTTP) locked(w http.ResponseWriter, ip string) bool {
	locked, retry := h.Limiter.Check(ip)
	if !locked {
		return false
	}
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	reply(w, 429, map[string]any{"error": fmt.Sprintf("Too many failed attempts. Try again in %ds. %s", retry, resetHint), "retryAfter": retry, "resetHint": resetHint})
	return true
}
func (h *HTTP) Login(w http.ResponseWriter, r *http.Request) {
	ip := peer(r)
	if h.locked(w, ip) {
		return
	}
	var raw json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&raw); err != nil || string(raw) == "null" {
		reply(w, 500, map[string]string{"error": "Invalid JSON body"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		reply(w, 500, map[string]string{"error": "Invalid JSON body"})
		return
	}
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	password, _ := body["password"].(string)
	settings, err := h.Settings(r.Context())
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to load settings"})
		return
	}
	if text(settings, "authMode") == "oidc" && configured(settings) {
		reply(w, 403, map[string]string{"error": "Password login is disabled. Use OIDC sign in."})
		return
	}
	hash := text(settings, "password")
	if hash == "" && !h.InitialValid {
		w.Header().Set("Cache-Control", "no-store")
		reply(w, 503, map[string]string{"error": "INITIAL_PASSWORD must be 123456 or a strong value before login"})
		return
	}
	if !VerifyPassword(password, hash, h.Initial) {
		left := h.Limiter.Fail(ip)
		if h.locked(w, ip) {
			return
		}
		reply(w, 401, map[string]any{"error": fmt.Sprintf("Invalid password. %d attempt(s) left before lockout.", left), "remainingBeforeLock": left})
		return
	}
	grant := hash == "" && IsPlaceholder(h.Initial) && !local(r)
	var token string
	if grant {
		token, err = h.Sessions.CreatePasswordChange()
	} else {
		token, err = h.Sessions.Create(nil)
	}
	if err != nil {
		reply(w, 500, map[string]string{"error": "Failed to create session"})
		return
	}
	h.Limiter.Success(ip)
	http.SetCookie(w, SessionCookie(r, token, grant, h.ForceSecure))
	w.Header().Set("Cache-Control", "no-store")
	reply(w, 200, map[string]any{"success": true, "mustChangePassword": grant})
}
func (h *HTTP) Logout(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{"auth_token", "oidc_state", "oidc_nonce", "oidc_code_verifier"} {
		http.SetCookie(w, &http.Cookie{Name: name, Path: "/", MaxAge: -1})
	}
	w.Header().Set("Cache-Control", "no-store")
	reply(w, 200, map[string]bool{"success": true})
}
func (h *HTTP) Status(w http.ResponseWriter, r *http.Request) {
	settings, err := h.Settings(r.Context())
	if err != nil {
		settings = map[string]any{}
	}
	session := h.session(r)
	if err != nil {
		session = nil
	}
	name := strings.TrimSpace(text(session, "oidcName"))
	email := strings.TrimSpace(text(session, "oidcEmail"))
	oidc, _ := session["oidc"].(bool)
	display := name
	if display == "" {
		display = email
	}
	method := "Password"
	if oidc {
		method = "OIDC"
	}
	if display == "" {
		display = method + " user"
	}
	mode := text(settings, "authMode")
	if mode == "" {
		mode = "password"
	}
	label := strings.TrimSpace(text(settings, "oidcLoginLabel"))
	if label == "" {
		label = "Sign in with OIDC"
	}
	var nameValue, emailValue any
	if name != "" {
		nameValue = name
	}
	if email != "" {
		emailValue = email
	}
	reply(w, 200, map[string]any{"requireLogin": settings["requireLogin"] != false, "authMode": mode, "oidcConfigured": configured(settings), "oidcLoginLabel": label, "hasPassword": text(settings, "password") != "", "displayName": display, "loginMethod": method, "oidcName": nameValue, "oidcEmail": emailValue, "oidcLogin": oidc})
}
func (h *HTTP) Check(w http.ResponseWriter, r *http.Request) {
	settings, err := h.Settings(r.Context())
	if err == nil && settings["requireLogin"] == false {
		reply(w, 200, map[string]bool{"authenticated": true})
		return
	}
	if h.session(r) == nil {
		reply(w, 401, map[string]string{"error": "Unauthorized"})
		return
	}
	if err == nil && h.CheckConfigured && h.session(r) != nil {
		reply(w, 200, map[string]bool{"authenticated": true})
		return
	}
	reply(w, 401, map[string]bool{"authenticated": false})
}

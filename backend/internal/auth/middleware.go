package auth

import (
	"net/http"
	"strings"
)

// isPublicAPIPath reproduz PUBLIC_API_PATHS do dashboardGuard.
func isPublicAPIPath(path string) bool {
	for _, p := range []string{"/api/health", "/api/ready", "/api/init", "/api/locale", "/api/auth/login", "/api/auth/logout", "/api/auth/status", "/api/auth/oidc", "/api/version"} {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

// isLocalOnlyPath reproduz LOCAL_ONLY_PATHS.
func isLocalOnlyPath(path string) bool {
	for _, p := range []string{"/api/oauth/cursor/auto-import", "/api/oauth/kiro/auto-import", "/api/oauth/zed/auto-import", "/api/auth/reset-password", "/api/headroom/start", "/api/headroom/stop", "/api/headroom/proxy"} {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// isAlwaysProtected reproduz ALWAYS_PROTECTED.
func isAlwaysProtected(path string) bool {
	for _, p := range []string{"/api/shutdown", "/api/settings/database", "/api/version/shutdown", "/api/version/update", "/api/oauth/cursor/auto-import", "/api/oauth/kiro/auto-import", "/api/oauth/zed/auto-import"} {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

// isPublicLLMAPIPath reproduz PUBLIC_PREFIXES.
func isPublicLLMAPIPath(path string) bool {
	for _, p := range []string{"/v1", "/v1beta", "/api/v1", "/api/v1beta", "/codex"} {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}

func (h *HTTP) hasValidCLIToken(r *http.Request) bool {
	token := r.Header.Get("X-9r-cli-token")
	if token == "" {
		return false
	}
	return token == h.CLIToken()
}

func (h *HTTP) canAccessLocalOnlyRoute(r *http.Request) bool {
	if h.hasValidCLIToken(r) {
		return true
	}
	ok, _ := h.authenticated(r)
	return ok && local(r)
}

func extractAPIKey(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	if v := r.Header.Get("x-api-key"); v != "" {
		return strings.TrimSpace(v)
	}
	if v := r.Header.Get("x-goog-api-key"); v != "" {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(r.URL.Query().Get("key"))
}

func (h *HTTP) canAccessPublicLLMAPI(r *http.Request) bool {
	if local(r) {
		return true
	}
	if h.hasValidCLIToken(r) {
		return true
	}
	// Validação real de API key.
	if h.ValidateAPIKey != nil {
		key := extractAPIKey(r)
		if key != "" {
			if rec, _ := h.ValidateAPIKey(r.Context(), key); rec != nil {
				return true
			}
		}
	}
	return false
}

// Middleware reproduz o dashboardGuard para /api/* (deny-by-default) e para
// os prefixos públicos de LLM. Rotas de página não são tratadas aqui.
func (h *HTTP) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/") && !isPublicLLMAPIPath(path) {
			next.ServeHTTP(w, r)
			return
		}
		if isLocalOnlyPath(path) {
			if !h.canAccessLocalOnlyRoute(r) {
				w.Header().Set("Cache-Control", "no-store")
				reply(w, http.StatusForbidden, map[string]string{"error": "Local only: CLI token required"})
				return
			}
		}
		if isAlwaysProtected(path) {
			if !h.hasValidCLIToken(r) {
				ok, _ := h.authenticated(r)
				if !ok {
					w.Header().Set("Cache-Control", "no-store")
					reply(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
					return
				}
			}
		}
		if isPublicLLMAPIPath(path) {
			if !h.canAccessPublicLLMAPI(r) {
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Access-Control-Allow-Origin", "*")
				reply(w, http.StatusUnauthorized, map[string]string{"error": "API key required for remote API access"})
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if !strings.HasPrefix(path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if isPublicAPIPath(path) {
			next.ServeHTTP(w, r)
			return
		}
		if h.hasValidCLIToken(r) {
			next.ServeHTTP(w, r)
			return
		}
		if ok, _ := h.authenticated(r); ok {
			next.ServeHTTP(w, r)
			return
		}
		if r.Method == http.MethodPatch && path == "/api/settings" && h.grantOnly(r) {
			next.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	})
}

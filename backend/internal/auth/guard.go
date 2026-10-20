package auth

import "net/http"

// guardAutenticado reproduz isAuthenticated do dashboardGuard: sessão JWT ou
// requireLogin=false. Token CLI é validado separadamente pelos guards.
func (h *HTTP) authenticated(r *http.Request) (bool, error) {
	if h.session(r) != nil {
		return true, nil
	}
	settings, err := h.Settings(r.Context())
	if err != nil {
		return false, err
	}
	return settings["requireLogin"] == false, nil
}

// SettingsGuard reproduz o dashboardGuard para /api/settings:
// PATCH com grant de troca de senha passa; demais chamadas exigem sessão,
// token CLI ou requireLogin=false.
func (h *HTTP) SettingsGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.hasValidCLIToken(r) {
			next(w, r)
			return
		}
		if r.Method == http.MethodPatch && h.grantOnly(r) {
			next(w, r)
			return
		}
		ok, err := h.authenticated(r)
		if err == nil && ok {
			next(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		reply(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
	}
}

// LocalOnlyGuard reproduz canAccessLocalOnlyRoute para /api/auth/reset-password:
// operador local autenticado ou token CLI válido passa.
func (h *HTTP) LocalOnlyGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.hasValidCLIToken(r) {
			next(w, r)
			return
		}
		ok, err := h.authenticated(r)
		if err == nil && ok && local(r) {
			next(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		// Mensagem idêntica à do dashboardGuard.
		reply(w, http.StatusForbidden, map[string]string{"error": "Local only: CLI token required"})
	}
}

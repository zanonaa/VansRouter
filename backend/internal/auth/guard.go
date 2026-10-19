package auth

import "net/http"

// guardAutenticado reproduz isAuthenticated do dashboardGuard: sessão JWT ou
// requireLogin=false. O token CLI (x-9r-cli-token/machineId) ainda não está
// implementado; chamadas que dependem dele serão recusadas até o lote próprio.
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
// operador local autenticado passa; token CLI ainda não implementado.
func (h *HTTP) LocalOnlyGuard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

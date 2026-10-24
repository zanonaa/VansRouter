package auth

import (
	"context"
	"net/http"
)

// VerifyDashboardPassword sempre usa o hash persistido quando presente;
// INITIAL_PASSWORD só é aceito antes da primeira definição de senha.
func (h *HTTP) VerifyDashboardPassword(ctx context.Context, password string) bool {
	settings, err := h.Settings(ctx)
	if err != nil {
		return false
	}
	hash := text(settings, "password")
	if hash == "" && !h.InitialValid {
		return false
	}
	return VerifyPassword(password, hash, h.Initial)
}
func (h *HTTP) ValidCLIToken(r *http.Request) bool { return h.hasValidCLIToken(r) }

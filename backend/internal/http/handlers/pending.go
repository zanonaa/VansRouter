package handlers

import "net/http"

// Pending registra contratos ainda sem executor portado. A resposta é explícita
// e nunca simula sucesso.
func Pending(w http.ResponseWriter, r *http.Request) {
	nodeReply(w, 501, map[string]any{"error": "Contract not fully ported", "path": r.URL.Path, "method": r.Method})
}

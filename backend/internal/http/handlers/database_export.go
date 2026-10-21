package handlers

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

type DatabaseExport struct {
	DB             *sql.DB
	VerifyPassword func(string) bool
	Settings       func(context.Context) (map[string]any, error)
	ValidCLI       func(*http.Request) bool
}

func (d DatabaseExport) cli(r *http.Request) bool { return d.ValidCLI != nil && d.ValidCLI(r) }
func (d DatabaseExport) Get(w http.ResponseWriter, r *http.Request) {
	if !d.cli(r) && (d.VerifyPassword == nil || !d.VerifyPassword(r.Header.Get("X-9r-password"))) {
		nodeReply(w, 401, map[string]string{"error": "Invalid password"})
		return
	}
	payload, err := database.ExportTransfer(r.Context(), d.DB)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to export database"})
		return
	}
	nodeReply(w, 200, payload)
}
func (d DatabaseExport) Post(w http.ResponseWriter, r *http.Request) {
	payload, err := poolInput(r)
	if err != nil || payload == nil {
		nodeReply(w, 400, map[string]string{"error": "Invalid database payload"})
		return
	}
	password, _ := payload["password"].(string)
	delete(payload, "password")
	if !d.cli(r) && (d.VerifyPassword == nil || !d.VerifyPassword(password)) {
		nodeReply(w, 401, map[string]string{"error": "Invalid password"})
		return
	}
	if err = database.ImportTransfer(r.Context(), d.DB, payload); err != nil {
		nodeReply(w, 400, map[string]string{"error": err.Error()})
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}

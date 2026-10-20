package handlers

import (
	"net/http"
	"os"
	"time"
)

// Shutdown não encerra processos por nome; sinaliza exclusivamente este servidor.
// A barreira de sessão/CLI é aplicada pelo middleware antes deste handler.
type Shutdown struct{ Request func() }

func (s Shutdown) Post(w http.ResponseWriter, r *http.Request) {
	if os.Getenv("NODE_ENV") == "production" {
		nodeReply(w, 403, map[string]any{"success": false, "message": "Not allowed in production"})
		return
	}
	secret := os.Getenv("SHUTDOWN_SECRET")
	if secret == "" || r.Header.Get("Authorization") != "Bearer "+secret {
		nodeReply(w, 401, map[string]any{"success": false, "message": "Unauthorized"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "message": "Shutting down..."})
	if s.Request != nil {
		time.AfterFunc(500*time.Millisecond, s.Request)
	}
}

package handlers

// Rotas /api/nordvpn/*: token (login), países, CRUD de túneis, health e
// rotação. Autenticadas pelo mesmo token CLI do dashboard de pools.
import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// NordVPNRoutes agrunta os handlers de NordVPN.
type NordVPNRoutes struct{}

// NewNordVPNRoutes devolve as rotas ligadas ao manager global.
func NewNordVPNRoutes() *NordVPNRoutes { return &NordVPNRoutes{} }

func (n *NordVPNRoutes) SetToken(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]any{"error": "Invalid JSON body"})
		return
	}
	token := strings.TrimSpace(ttsString(body["token"]))
	if token == "" {
		nodeReply(w, 400, map[string]any{"error": "token required"})
		return
	}
	if err := NordSetToken(token); err != nil {
		nodeReply(w, 401, map[string]any{"error": err.Error()})
		return
	}
	nodeReply(w, 200, map[string]any{"ok": true})
}

func (n *NordVPNRoutes) Countries(w http.ResponseWriter, r *http.Request) {
	countries, err := NordCountries(r.Context())
	if err != nil {
		nodeReply(w, 502, map[string]any{"error": err.Error()})
		return
	}
	nodeReply(w, 200, map[string]any{"countries": countries})
}

func (n *NordVPNRoutes) CreateTunnels(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		nodeReply(w, 400, map[string]any{"error": "Invalid JSON body"})
		return
	}
	request := NordCreateRequest{
		Countries: stringSlice(body["countries"]),
		Replicas:  int(pxpipeNum(body["replicas"])),
	}
	created, err := NordCreate(r.Context(), request)
	if err != nil {
		nodeReply(w, 502, map[string]any{"error": err.Error(), "created": created})
		return
	}
	nodeReply(w, 200, map[string]any{"tunnels": created})
}

func (n *NordVPNRoutes) ListTunnels(w http.ResponseWriter, r *http.Request) {
	nodeReply(w, 200, map[string]any{"tunnels": NordList()})
}

func (n *NordVPNRoutes) Health(w http.ResponseWriter, r *http.Request) {
	nodeReply(w, 200, map[string]any{"tunnels": NordHealth(r.Context())})
}

func (n *NordVPNRoutes) Rotate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	status, err := NordRotate(r.Context(), id)
	if err != nil {
		nodeReply(w, 502, map[string]any{"error": err.Error()})
		return
	}
	nodeReply(w, 200, status)
}

func (n *NordVPNRoutes) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if !NordDelete(id) {
		nodeReply(w, 404, map[string]any{"error": "tunnel not found"})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true})
}

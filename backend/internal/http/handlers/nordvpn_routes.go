package handlers

// Rotas /api/nordvpn/*: token (login), países, CRUD de túneis, health e
// rotação. Autenticadas pelo mesmo token CLI do dashboard de pools.
import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/go-chi/chi/v5"
)

// NordVPNRoutes agrunta os handlers de NordVPN.
type nordPendingLogin struct {
	client            *NordClient
	attempt, verifier string
	expires           time.Time
}

type NordVPNRoutes struct {
	mu      sync.Mutex
	pending map[string]nordPendingLogin
}

// NewNordVPNRoutes devolve as rotas ligadas ao manager global.
func NewNordVPNRoutes() *NordVPNRoutes { return &NordVPNRoutes{} }

// OAuth state is one-use, short-lived and contains the PKCE verifier only
// server-side. The Nord Account login (including Google/MFA) stays in browser.
func (n *NordVPNRoutes) OAuthStart(w http.ResponseWriter, r *http.Request) {
	verifier, challenge, err := NordPKCE()
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": "Could not start login"})
		return
	}
	client := &NordClient{}
	uri, attempt, err := client.LoginURI(r.Context(), challenge)
	if err != nil {
		nodeReply(w, 502, map[string]any{"error": "Nord Account login unavailable"})
		return
	}
	n.mu.Lock()
	if n.pending == nil {
		n.pending = map[string]nordPendingLogin{}
	}
	for key, login := range n.pending {
		if time.Now().After(login.expires) {
			delete(n.pending, key)
		}
	}
	if len(n.pending) >= 32 {
		n.mu.Unlock()
		nodeReply(w, 429, map[string]any{"error": "Too many pending logins"})
		return
	}
	state := uuid.NewString()
	n.pending[state] = nordPendingLogin{client: client, attempt: attempt, verifier: verifier, expires: time.Now().Add(10 * time.Minute)}
	n.mu.Unlock()
	nodeReply(w, 200, map[string]any{"state": state, "url": uri})
}

func (n *NordVPNRoutes) OAuthComplete(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		nodeReply(w, 400, map[string]any{"error": "Invalid JSON body"})
		return
	}
	state, exchange := ttsString(body["state"]), strings.TrimSpace(ttsString(body["exchangeToken"]))
	n.mu.Lock()
	login, ok := n.pending[state]
	delete(n.pending, state)
	n.mu.Unlock()
	if !ok || time.Now().After(login.expires) || exchange == "" {
		nodeReply(w, 400, map[string]any{"error": "Login expired or invalid; start again"})
		return
	}
	if err := login.client.LoginExchange(r.Context(), login.attempt, login.verifier, exchange); err != nil {
		nodeReply(w, 401, map[string]any{"error": "Nord Account rejected login; start again"})
		return
	}
	if _, err := login.client.Credentials(r.Context()); err != nil {
		nodeReply(w, 401, map[string]any{"error": "Could not retrieve NordLynx credentials"})
		return
	}
	nordManager.mu.Lock()
	nordManager.client = login.client
	nordManager.mu.Unlock()
	nodeReply(w, 200, map[string]any{"ok": true})
}

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

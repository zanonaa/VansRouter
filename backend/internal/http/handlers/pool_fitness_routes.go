package handlers

// Porta das rotas src/app/api/proxy-pools/fitness/* e /api/proxy-pools/[id]/fitness/clear.
import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"strings"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

// PoolFitnessRoutes expõe snapshot/clear com service read-through.
type PoolFitnessRoutes struct {
	Service *PoolFitnessService
}

func NewPoolFitnessRoutes(store database.ProxyFitness) *PoolFitnessRoutes {
	return &PoolFitnessRoutes{Service: &PoolFitnessService{Store: store}}
}

func (p *PoolFitnessRoutes) Get(w http.ResponseWriter, r *http.Request) {
	snapshot := p.Service.Snapshot(r.Context(), time.Now().UnixMilli())
	if snapshot == nil {
		nodeReply(w, 500, map[string]any{"error": "Failed to read proxy fitness"})
		return
	}
	nodeReply(w, 200, map[string]any{"pools": snapshot})
}

// ClearAll: provider opcional no body.
func (p *PoolFitnessRoutes) ClearAll(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil || body == nil {
		body = map[string]any{}
	}
	provider := ""
	if raw, ok := body["provider"].(string); ok && strings.TrimSpace(raw) != "" {
		provider = strings.TrimSpace(raw)
	}
	if !p.Service.ClearAllPoolUnfit(r.Context(), provider) {
		nodeReply(w, 500, map[string]any{"error": "Failed to clear proxy fitness"})
		return
	}
	nodeReply(w, 200, map[string]any{"ok": true, "provider": nilOrNilStringValue(provider)})
}

func nilOrNilStringValue(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// ClearByID: POST /api/proxy-pools/{id}/fitness/clear com {scope}.
func (p *PoolFitnessRoutes) ClearByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	body, err := poolInput(r)
	if err != nil || body == nil {
		body = map[string]any{}
	}
	scope := ""
	if raw, ok := body["scope"].(string); ok {
		scope = strings.TrimSpace(raw)
	}
	if id == "" || scope == "" {
		nodeReply(w, 400, map[string]any{"error": "pool id and scope are required"})
		return
	}
	if !p.Service.ClearPoolUnfit(r.Context(), id, scope) {
		nodeReply(w, 500, map[string]any{"error": "Failed to clear pool fitness"})
		return
	}
	nodeReply(w, 200, map[string]any{"ok": true, "poolId": id, "scope": scope})
}

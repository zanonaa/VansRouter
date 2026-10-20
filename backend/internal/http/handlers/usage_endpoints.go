package handlers

import (
	"context"
	"net/http"
)

type UsageStore interface {
	History(context.Context, string, string, string, string) ([]map[string]any, error)
	Chart(context.Context, string) ([]map[string]any, error)
	DistinctProviders(context.Context) ([]string, error)
}
type UsageEndpoints struct {
	Usage UsageStore
	Nodes NodeStore
}

func (h UsageEndpoints) History(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	history, err := h.Usage.History(r.Context(), query.Get("provider"), query.Get("model"), query.Get("startDate"), query.Get("endDate"))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch usage history"})
		return
	}
	if history == nil {
		history = []map[string]any{}
	}
	nodeReply(w, 200, history)
}
func (h UsageEndpoints) Chart(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "7d"
	}
	switch period {
	case "today", "24h", "7d", "30d", "60d", "all":
	default:
		nodeReply(w, 400, map[string]string{"error": "Invalid period"})
		return
	}
	data, err := h.Usage.Chart(r.Context(), period)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch chart data"})
		return
	}
	if data == nil {
		data = []map[string]any{}
	}
	nodeReply(w, 200, data)
}
func (h UsageEndpoints) Providers(w http.ResponseWriter, r *http.Request) {
	ids, err := h.Usage.DistinctProviders(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch providers"})
		return
	}
	names := map[string]any{}
	if nodes, err := h.Nodes.List(r.Context(), ""); err == nil {
		for _, node := range nodes {
			id, _ := node["id"].(string)
			if nodeTruthy(node["name"]) && id != "" {
				names[id] = node["name"]
			}
		}
	}
	providers := []map[string]any{}
	for _, id := range ids {
		name := any(id)
		if value, ok := names[id]; ok {
			name = value
		}
		providers = append(providers, map[string]any{"id": id, "name": name})
	}
	nodeReply(w, 200, map[string]any{"providers": providers})
}

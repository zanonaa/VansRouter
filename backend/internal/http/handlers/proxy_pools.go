package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type PoolStore interface {
	List(context.Context, *bool, string) ([]map[string]any, error)
	Get(context.Context, string) (map[string]any, error)
	Create(context.Context, map[string]any) (map[string]any, error)
	Update(context.Context, string, map[string]any) (map[string]any, error)
	Delete(context.Context, string) (map[string]any, error)
}
type ProxyPools struct {
	Store       PoolStore
	Connections NodeConnections
}

func poolInput(r *http.Request) (map[string]any, error) {
	var body map[string]any
	d := json.NewDecoder(r.Body)
	if err := d.Decode(&body); err != nil {
		return nil, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, io.ErrUnexpectedEOF
	}
	return body, nil
}
func normalizePool(body map[string]any, create bool) (map[string]any, string) {
	updates := map[string]any{}
	for _, key := range []string{"name", "proxyUrl", "noProxy"} {
		_, present := body[key]
		if !present && !create {
			continue
		}
		value, _ := body[key].(string)
		value = strings.TrimSpace(value)
		if value == "" && key != "noProxy" {
			if key == "name" {
				return nil, "Name is required"
			}
			return nil, "Proxy URL is required"
		}
		updates[key] = value
	}
	for _, key := range []string{"isActive", "strictProxy"} {
		value, present := body[key]
		if present || create {
			updates[key] = value == true
			if key == "isActive" && !present {
				updates[key] = true
			}
		}
	}
	if _, present := body["type"]; present || create {
		kind, _ := body["type"].(string)
		switch kind {
		case "http", "vercel", "cloudflare", "deno":
		default:
			kind = "http"
		}
		updates["type"] = kind
	}
	return updates, ""
}
func poolUsage(connections []map[string]any) map[string]int {
	counts := map[string]int{}
	for _, connection := range connections {
		data, _ := connection["providerSpecificData"].(map[string]any)
		seen := map[string]bool{}
		if id, ok := data["proxyPoolId"].(string); ok && id != "" {
			seen[id] = true
		}
		if ids, ok := data["proxyPoolIds"].([]any); ok {
			for _, value := range ids {
				if id, ok := value.(string); ok && id != "" {
					seen[id] = true
				}
			}
		}
		for id := range seen {
			counts[id]++
		}
	}
	return counts
}
func (h ProxyPools) List(w http.ResponseWriter, r *http.Request) {
	var active *bool
	if value := r.URL.Query().Get("isActive"); value == "true" || value == "false" {
		v := value == "true"
		active = &v
	}
	pools, err := h.Store.List(r.Context(), active, "")
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch proxy pools"})
		return
	}
	if pools == nil {
		pools = []map[string]any{}
	}
	if r.URL.Query().Get("includeUsage") == "true" {
		connections, err := h.Connections.List(r.Context(), "", nil)
		if err != nil {
			nodeReply(w, 500, map[string]string{"error": "Failed to fetch proxy pools"})
			return
		}
		usage := poolUsage(connections)
		for _, pool := range pools {
			id, _ := pool["id"].(string)
			pool["boundConnectionCount"] = usage[id]
		}
	}
	nodeReply(w, 200, map[string]any{"proxyPools": pools})
}
func (h ProxyPools) Create(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to create proxy pool"}) }
	body, err := poolInput(r)
	if err != nil {
		fail()
		return
	}
	updates, message := normalizePool(body, true)
	if message != "" {
		nodeReply(w, 400, map[string]string{"error": message})
		return
	}
	pool, err := h.Store.Create(r.Context(), updates)
	if err != nil {
		fail()
		return
	}
	nodeReply(w, 201, map[string]any{"proxyPool": pool})
}
func (h ProxyPools) Get(w http.ResponseWriter, r *http.Request) {
	pool, err := h.Store.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch proxy pool"})
		return
	}
	if pool == nil {
		nodeReply(w, 404, map[string]string{"error": "Proxy pool not found"})
		return
	}
	nodeReply(w, 200, map[string]any{"proxyPool": pool})
}
func (h ProxyPools) Update(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to update proxy pool"}) }
	id := chi.URLParam(r, "id")
	existing, err := h.Store.Get(r.Context(), id)
	if err != nil {
		fail()
		return
	}
	if existing == nil {
		nodeReply(w, 404, map[string]string{"error": "Proxy pool not found"})
		return
	}
	body, err := poolInput(r)
	if err != nil || body == nil {
		fail()
		return
	}
	updates, message := normalizePool(body, false)
	if message != "" {
		nodeReply(w, 400, map[string]string{"error": message})
		return
	}
	pool, err := h.Store.Update(r.Context(), id, updates)
	if err != nil {
		fail()
		return
	}
	nodeReply(w, 200, map[string]any{"proxyPool": pool})
}
func (h ProxyPools) Delete(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to delete proxy pool"}) }
	id := chi.URLParam(r, "id")
	existing, err := h.Store.Get(r.Context(), id)
	if err != nil {
		fail()
		return
	}
	if existing == nil {
		nodeReply(w, 404, map[string]string{"error": "Proxy pool not found"})
		return
	}
	connections, err := h.Connections.List(r.Context(), "", nil)
	if err != nil {
		fail()
		return
	}
	count := poolUsage(connections)[id]
	if count > 0 {
		nodeReply(w, 409, map[string]any{"error": "Proxy pool is currently in use", "boundConnectionCount": count})
		return
	}
	if err = deleteProxyPool(r.Context(), h.Store, id); err != nil {
		fail()
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}

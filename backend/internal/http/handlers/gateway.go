package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Gateway struct {
	DB     *sql.DB
	Client *http.Client
}

func (g *Gateway) key(r *http.Request) bool {
	key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if key == "" {
		key = r.Header.Get("x-api-key")
	}
	if key == "" {
		return false
	}
	var active int
	return g.DB.QueryRowContext(r.Context(), "SELECT isActive FROM apiKeys WHERE key=?", key).Scan(&active) == nil && active == 1
}
func (g *Gateway) connection(ctx context.Context, provider string) (map[string]any, error) {
	var id, raw string
	err := g.DB.QueryRowContext(ctx, "SELECT id,data FROM providerConnections WHERE provider=? AND isActive=1 ORDER BY priority ASC LIMIT 1", provider).Scan(&id, &raw)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	json.Unmarshal([]byte(raw), &data)
	data["id"] = id
	return data, nil
}
func (g *Gateway) proxy(w http.ResponseWriter, r *http.Request, url string, headers map[string]string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		nodeReply(w, 400, map[string]any{"error": err.Error()})
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), r.Method, url, bytes.NewReader(body))
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	for k, v := range headers {
		request.Header.Set(k, v)
	}
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		nodeReply(w, 502, map[string]any{"error": err.Error()})
		return
	}
	defer response.Body.Close()
	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
}
func (g *Gateway) Chat(w http.ResponseWriter, r *http.Request) {
	if !g.key(r) {
		nodeReply(w, 401, map[string]any{"error": "API key required"})
		return
	}
	connection, err := g.connection(r.Context(), "openai")
	if err != nil {
		nodeReply(w, 404, map[string]any{"error": "No active connection"})
		return
	}
	g.proxy(w, r, "https://api.openai.com/v1/chat/completions", map[string]string{"Authorization": "Bearer " + ttsString(connection["apiKey"]), "Content-Type": "application/json"})
}
func (g *Gateway) Embeddings(w http.ResponseWriter, r *http.Request) {
	if !g.key(r) {
		nodeReply(w, 401, map[string]any{"error": "API key required"})
		return
	}
	connection, err := g.connection(r.Context(), "openai")
	if err != nil {
		nodeReply(w, 404, map[string]any{"error": "No active connection"})
		return
	}
	g.proxy(w, r, "https://api.openai.com/v1/embeddings", map[string]string{"Authorization": "Bearer " + ttsString(connection["apiKey"]), "Content-Type": "application/json"})
}
func (g *Gateway) Images(w http.ResponseWriter, r *http.Request) {
	if !g.key(r) {
		nodeReply(w, 401, map[string]any{"error": "API key required"})
		return
	}
	connection, err := g.connection(r.Context(), "openai")
	if err != nil {
		nodeReply(w, 404, map[string]any{"error": "No active connection"})
		return
	}
	g.proxy(w, r, "https://api.openai.com/v1/images/generations", map[string]string{"Authorization": "Bearer " + ttsString(connection["apiKey"]), "Content-Type": "application/json"})
}
func (g *Gateway) Models(w http.ResponseWriter, r *http.Request) {
	if !g.key(r) {
		nodeReply(w, 401, map[string]any{"error": "API key required"})
		return
	}
	connection, err := g.connection(r.Context(), "openai")
	if err != nil {
		nodeReply(w, 404, map[string]any{"error": "No active connection"})
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), "GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		nodeReply(w, 500, map[string]any{"error": err.Error()})
		return
	}
	request.Header.Set("Authorization", "Bearer "+ttsString(connection["apiKey"]))
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		nodeReply(w, 502, map[string]any{"error": err.Error()})
		return
	}
	defer response.Body.Close()
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
}
func (g *Gateway) NotPorted(w http.ResponseWriter, r *http.Request) {
	if !g.key(r) {
		nodeReply(w, 401, map[string]any{"error": "API key required"})
		return
	}
	nodeReply(w, 501, map[string]any{"error": "Gateway executor not ported", "path": r.URL.Path})
}

var _ = chi.URLParam

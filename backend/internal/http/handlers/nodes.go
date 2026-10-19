package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type NodeStore interface {
	List(context.Context, string) ([]map[string]any, error)
	Create(context.Context, map[string]any) (map[string]any, error)
}
type NodeMutations interface {
	Get(context.Context, string) (map[string]any, error)
	Update(context.Context, string, map[string]any) (map[string]any, error)
	Delete(context.Context, string) (map[string]any, error)
}
type NodeConnections interface {
	List(context.Context, string, *bool) ([]map[string]any, error)
	Update(context.Context, string, map[string]any) (map[string]any, error)
	DeleteByProvider(context.Context, string) (int64, error)
}
type Nodes struct {
	Store       NodeStore
	Mutations   NodeMutations
	Connections NodeConnections
}

func nodeReply(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func (h Nodes) List(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.Store.List(r.Context(), "")
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch provider nodes"})
		return
	}
	if nodes == nil {
		nodes = []map[string]any{}
	}
	nodeReply(w, 200, map[string]any{"nodes": nodes})
}
func (h Nodes) Create(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to create provider node"}) }
	var body map[string]any
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&body); err != nil || body == nil {
		fail()
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		fail()
		return
	}
	// Optional chaining trim() on a non-string throws in the source handler.
	field := func(key string) (string, bool) {
		v, exists := body[key]
		if !exists || v == nil {
			return "", true
		}
		s, ok := v.(string)
		return s, ok
	}
	name, ok := field("name")
	if !ok {
		fail()
		return
	}
	name = strings.TrimSpace(name)
	if name == "" {
		nodeReply(w, 400, map[string]string{"error": "Name is required"})
		return
	}
	prefix, ok := field("prefix")
	if !ok {
		fail()
		return
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		nodeReply(w, 400, map[string]string{"error": "Prefix is required"})
		return
	}
	kind := body["type"]
	if !nodeTruthy(kind) {
		kind = "openai-compatible"
	}
	kindString, _ := kind.(string)
	apiType, _ := body["apiType"].(string)
	if kindString == "openai-compatible" && apiType != "chat" && apiType != "responses" {
		nodeReply(w, 400, map[string]string{"error": "Invalid OpenAI compatible API type"})
		return
	}
	if kindString != "openai-compatible" && kindString != "anthropic-compatible" && kindString != "custom-embedding" {
		nodeReply(w, 400, map[string]string{"error": "Invalid provider node type"})
		return
	}
	base := "https://api.openai.com/v1"
	if kindString == "anthropic-compatible" {
		base = "https://api.anthropic.com/v1"
	}
	if nodeTruthy(body["baseUrl"]) {
		value, ok := body["baseUrl"].(string)
		if !ok {
			fail()
			return
		}
		base = value
	}
	base = strings.TrimSpace(base)
	idPrefix := kindString + "-"
	if kindString == "openai-compatible" {
		idPrefix += apiType + "-"
	} else {
		base = strings.TrimSuffix(base, "/")
		if kindString == "custom-embedding" {
			base = strings.TrimSuffix(base, "/embeddings")
		} else {
			base = strings.TrimSuffix(base, "/messages")
			if !strings.HasSuffix(base, "/v1") {
				base += "/v1"
			}
		}
	}
	data := map[string]any{"id": idPrefix + uuid.NewString(), "type": kindString, "prefix": prefix, "baseUrl": base, "name": name}
	if kindString == "openai-compatible" {
		data["apiType"] = apiType
	}
	node, err := h.Store.Create(r.Context(), data)
	if err != nil {
		fail()
		return
	}
	nodeReply(w, 201, map[string]any{"node": node})
}
func nodeTruthy(v any) bool {
	switch value := v.(type) {
	case nil:
		return false
	case bool:
		return value
	case string:
		return value != ""
	case float64:
		return value != 0
	default:
		return true
	}
}

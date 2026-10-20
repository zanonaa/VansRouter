package handlers

import (
	"context"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type ComboStore interface {
	List(context.Context) ([]map[string]any, error)
	Get(context.Context, string) (map[string]any, error)
	ByName(context.Context, any) (map[string]any, error)
	Create(context.Context, map[string]any) (map[string]any, error)
	Update(context.Context, string, map[string]any) (map[string]any, error)
	Delete(context.Context, string) (bool, error)
}
type Combos struct{ Store ComboStore }

var comboName = regexp.MustCompile(`^[a-zA-Z0-9_.\-]+$`)

func contextLength(value any) (any, string) {
	if value == nil || value == "" {
		return nil, ""
	}
	var n float64
	var err error
	switch v := value.(type) {
	case float64:
		n = v
	case bool:
		if v {
			n = 1
		}
	case string:
		if strings.TrimSpace(v) == "" {
			n = 0
		} else {
			text := strings.TrimSpace(v)
			if len(text) > 2 && text[0] == '0' && strings.ContainsAny(string(text[1]), "xXbBoO") {
				parsed, parseErr := strconv.ParseUint(text, 0, 64)
				n = float64(parsed)
				err = parseErr
			} else {
				n, err = strconv.ParseFloat(text, 64)
			}
		}
	case []any:
		if len(v) == 0 {
			n = 0
		} else if len(v) == 1 {
			return contextLength(jsString(v))
		} else {
			err = strconv.ErrSyntax
		}
	default:
		err = strconv.ErrSyntax
	}
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) || math.Trunc(n) != n || n <= 0 {
		return nil, "context_length must be a positive integer"
	}
	if n > 2000000 {
		return nil, "context_length must not exceed 2000000"
	}
	return n, ""
}
func (h Combos) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.List(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch combos"})
		return
	}
	nodeReply(w, 200, map[string]any{"combos": list})
}
func (h Combos) Get(w http.ResponseWriter, r *http.Request) {
	value, err := h.Store.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch combo"})
		return
	}
	if value == nil {
		nodeReply(w, 404, map[string]string{"error": "Combo not found"})
		return
	}
	nodeReply(w, 200, value)
}
func (h Combos) Create(w http.ResponseWriter, r *http.Request) { h.write(w, r, true) }
func (h Combos) Update(w http.ResponseWriter, r *http.Request) { h.write(w, r, false) }
func (h Combos) write(w http.ResponseWriter, r *http.Request, create bool) {
	operation := "update"
	if create {
		operation = "create"
	}
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to " + operation + " combo"}) }
	reject := func(message string) { nodeReply(w, 400, map[string]string{"error": message}) }
	body, err := poolInput(r)
	if err != nil || body == nil {
		fail()
		return
	}
	name := body["name"]
	if create && !nodeTruthy(name) {
		reject("Name is required")
		return
	}
	id := chi.URLParam(r, "id")
	if nodeTruthy(name) {
		if !comboName.MatchString(jsString(name)) {
			reject("Name can only contain letters, numbers, -, _ and .")
			return
		}
		if create {
			if value, exists := body["context_length"]; exists {
				normalized, message := contextLength(value)
				if message != "" {
					reject(message)
					return
				}
				body["context_length"] = normalized
			}
		}
		existing, err := h.Store.ByName(r.Context(), name)
		if err != nil {
			fail()
			return
		}
		if existing != nil && (create || existing["id"] != id) {
			reject("Combo name already exists")
			return
		}
	}
	if value, exists := body["context_length"]; exists {
		normalized, message := contextLength(value)
		if message != "" {
			reject(message)
			return
		}
		body["context_length"] = normalized
	}
	var result map[string]any
	if create {
		models := body["models"]
		if !nodeTruthy(models) {
			models = []any{}
		}
		kind := body["kind"]
		if !nodeTruthy(kind) {
			kind = nil
		}
		result, err = h.Store.Create(r.Context(), map[string]any{"name": name, "models": models, "kind": kind, "context_length": body["context_length"]})
	} else {
		result, err = h.Store.Update(r.Context(), id, body)
	}
	if err != nil {
		fail()
		return
	}
	if result == nil {
		nodeReply(w, 404, map[string]string{"error": "Combo not found"})
		return
	}
	status := 200
	if create {
		status = 201
	}
	nodeReply(w, status, result)
}
func (h Combos) Delete(w http.ResponseWriter, r *http.Request) {
	deleted, err := h.Store.Delete(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to delete combo"})
		return
	}
	if !deleted {
		nodeReply(w, 404, map[string]string{"error": "Combo not found"})
		return
	}
	nodeReply(w, 200, map[string]bool{"success": true})
}

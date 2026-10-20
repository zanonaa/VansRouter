package handlers

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/database"
)

type LogStore interface {
	RecentLogs(context.Context, int, func() map[string]string) ([]string, error)
	RequestDetails(context.Context, database.RequestDetailsFilter) ([]any, int, error)
	RequestDetailByID(context.Context, string) (any, error)
}
type LogEndpoints struct {
	Store       LogStore
	Connections NodeConnections
}

func (h LogEndpoints) RecentLogs(w http.ResponseWriter, r *http.Request) {
	names := func() map[string]string {
		result := map[string]string{}
		if h.Connections != nil {
			if connections, err := h.Connections.List(r.Context(), "", nil); err == nil {
				for _, connection := range connections {
					id, _ := connection["id"].(string)
					value := connection["name"]
					if !nodeTruthy(value) {
						value = connection["email"]
					}
					if nodeTruthy(value) {
						result[id] = jsString(value)
					}
				}
			}
		}
		return result
	}
	logs, err := h.Store.RecentLogs(r.Context(), 200, names)
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch logs"})
		return
	}
	if logs == nil {
		logs = []string{}
	}
	nodeReply(w, 200, logs)
}

var parseIntPrefix = regexp.MustCompile(`^[+-]?[0-9]+`)

func queryInteger(value string, fallback int) int {
	value = strings.TrimSpace(value)
	sign := 1
	if strings.HasPrefix(value, "-") {
		sign = -1
	}
	unsigned := strings.TrimLeft(value, "+-")
	if strings.HasPrefix(strings.ToLower(unsigned), "0x") {
		if n, err := strconv.ParseInt(unsigned[2:], 16, 32); err == nil {
			return sign * int(n)
		}
	}
	prefix := parseIntPrefix.FindString(value)
	if prefix == "" {
		return fallback
	}
	n, err := strconv.ParseInt(prefix, 10, 32)
	if err != nil {
		if strings.HasPrefix(prefix, "-") {
			return -1
		}
		return int(^uint(0) >> 1)
	}
	return int(n)
}
func (h LogEndpoints) RequestDetails(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, size := queryInteger(q.Get("page"), 1), queryInteger(q.Get("pageSize"), 20)
	if page < 1 {
		nodeReply(w, 400, map[string]string{"error": "Page must be >= 1"})
		return
	}
	if size < 1 || size > 100 {
		nodeReply(w, 400, map[string]string{"error": "PageSize must be between 1 and 100"})
		return
	}
	details, total, err := h.Store.RequestDetails(r.Context(), database.RequestDetailsFilter{Page: page, PageSize: size, Provider: q.Get("provider"), Model: q.Get("model"), ConnectionID: q.Get("connectionId"), Status: q.Get("status"), Start: q.Get("startDate"), End: q.Get("endDate")})
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch request details"})
		return
	}
	redacted := []map[string]any{}
	for _, detail := range details {
		copy := jsSpread(detail)
		for _, key := range []string{"request", "providerRequest", "providerResponse", "response"} {
			if _, exists := copy[key]; exists {
				copy[key] = map[string]bool{"redacted": true}
			}
		}
		redacted = append(redacted, copy)
	}
	pages := (total + size - 1) / size
	nodeReply(w, 200, map[string]any{"details": redacted, "pagination": map[string]any{"page": page, "pageSize": size, "totalItems": total, "totalPages": pages, "hasNext": page < pages, "hasPrev": page > 1}})
}
func (h LogEndpoints) RequestDetailByID(w http.ResponseWriter, r *http.Request) {
	detail, err := h.Store.RequestDetailByID(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch detail"})
		return
	}
	if !nodeTruthy(detail) {
		nodeReply(w, 404, map[string]string{"error": "Not found"})
		return
	}
	nodeReply(w, 200, map[string]any{"detail": detail})
}

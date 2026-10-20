package handlers

import (
	"context"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf16"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type Overrides struct {
	Read  func(context.Context) (map[string]any, error)
	Write func(context.Context, map[string]any) (map[string]any, error)
}

var overrideName = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

func (o Overrides) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := o.Read(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to get overrides"})
		return
	}
	canonical := providers.ResolveAlias(chi.URLParam(r, "id"))
	all, _ := settings["providerOverrides"].(map[string]any)
	value, _ := all[canonical].(map[string]any)
	headers := value["headers"]
	if !nodeTruthy(headers) {
		headers = map[string]any{}
	}
	nodeReply(w, 200, map[string]any{"headers": headers, "builtinHeaders": providers.BuiltinHeaders(canonical)})
}
func (o Overrides) Put(w http.ResponseWriter, r *http.Request) {
	reject := func(message string) { nodeReply(w, 400, map[string]string{"error": message}) }
	body, err := poolInput(r)
	if err != nil {
		body = map[string]any{}
	}
	if body == nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save overrides"})
		return
	}
	clean := map[string]any{}
	if raw := body["headers"]; raw != nil {
		headers, ok := raw.(map[string]any)
		if !ok {
			reject("headers must be an object")
			return
		}
		count := 0
		for _, value := range headers {
			if value != nil && value != "" {
				count++
			}
		}
		if count > 20 {
			reject("Too many headers (max 20)")
			return
		}
		for name, value := range headers {
			if value == nil || value == "" {
				continue
			}
			if !overrideName.MatchString(name) {
				reject("Invalid header name: " + name)
				return
			}
			text, ok := value.(string)
			if !ok || strings.ContainsAny(text, "\r\n") {
				reject("Invalid value for header " + name)
				return
			}
			if len(utf16.Encode([]rune(text))) > 8192 {
				reject("Header " + name + " value too long (max 8192)")
				return
			}
			switch strings.ToLower(name) {
			case "host", "content-length", "content-type", "connection", "transfer-encoding", "authorization", "cookie":
				reject("Header " + name + " cannot be overridden")
				return
			}
			clean[name] = text
		}
	}
	settings, err := o.Read(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save overrides"})
		return
	}
	next := jsSpread(settings["providerOverrides"])
	canonical := providers.ResolveAlias(chi.URLParam(r, "id"))
	if len(clean) == 0 {
		delete(next, canonical)
	} else {
		next[canonical] = map[string]any{"headers": clean}
	}
	if _, err = o.Write(r.Context(), map[string]any{"providerOverrides": next}); err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to save overrides"})
		return
	}
	nodeReply(w, 200, map[string]any{"headers": clean})
}

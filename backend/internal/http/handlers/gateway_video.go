package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

// Video jobs belong to the creating upstream account. Polling must never fall
// back to a different account; creation must not retry ambiguous network/5xx errors.
func (g *Gateway) Video(w http.ResponseWriter, r *http.Request) {
	key, ok := g.modelAuth(w, r)
	if !ok {
		return
	}
	if !gatewayPermission(key, "allowedKinds", "video") {
		g.webError(w, 403, "Video generation requests are not allowed for this API key")
		return
	}
	provider := "xai"
	var connection map[string]any
	var raw []byte
	var err error
	requestID := chi.URLParam(r, "id")
	polling := r.Method == http.MethodGet
	preferred := r.Header.Get("x-connection-id")
	if polling {
		if requestID == "" {
			g.webError(w, 400, "Missing video request id")
			return
		}
		if preferred == "" {
			g.webError(w, 400, "Missing x-connection-id for video polling")
			return
		}
		connection, err = (database.Connections{DB: g.DB}).Get(r.Context(), preferred)
		if err != nil || connection == nil || connection["isActive"] != true {
			g.webError(w, 404, "Video connection is unavailable")
			return
		}
		provider = ttsString(connection["provider"])
		config := providers.WebConfigs()[provider].Video
		if config == nil {
			g.webError(w, 404, "Video connection is unavailable")
			return
		}
		queried := r.URL.Query().Get("provider")
		if queried != "" && queried != provider && queried != ttsString(config["provider"]) {
			g.webError(w, 400, "Video provider does not match x-connection-id")
			return
		}
	} else {
		raw, err = io.ReadAll(r.Body)
		if err != nil {
			g.webError(w, 400, "Invalid request body")
			return
		}
		if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
			var body map[string]any
			if json.Unmarshal(raw, &body) != nil || body == nil {
				g.webError(w, 400, "Invalid JSON body")
				return
			}
			if model := ttsString(body["model"]); strings.Contains(model, "/") {
				resolved, bare := splitTranslatorModel(model)
				if providers.WebConfigs()[resolved].Video == nil {
					g.webError(w, 400, "Provider '"+resolved+"' does not support video generation")
					return
				}
				provider = resolved
				body["model"] = bare
				raw, err = json.Marshal(body)
				if err != nil {
					g.webError(w, 400, "Invalid JSON body")
					return
				}
			}
		}
		if preferred != "" {
			connection, err = (database.Connections{DB: g.DB}).Get(r.Context(), preferred)
			if err != nil || connection == nil || connection["isActive"] != true || connection["provider"] != provider {
				connection = nil
			}
		}
		if connection == nil {
			connection, err = g.connection(r.Context(), provider)
		}
		if err != nil || connection == nil {
			g.webError(w, 400, "No credentials for provider: "+provider)
			return
		}
	}
	if !gatewayPermission(key, "allowedProviders", provider) {
		g.webError(w, 403, `Provider "`+provider+`" is not allowed for this API key`)
		return
	}
	config := providers.WebConfigs()[provider].Video
	target := strings.TrimRight(ttsString(config["baseUrl"]), "/")
	if polling {
		target += "/" + url.PathEscape(requestID)
	} else {
		action := strings.TrimPrefix(r.URL.Path, "/api/v1/videos/")
		switch action {
		case "generations", "edits", "extensions":
		default:
			g.webError(w, 400, "Unknown video action: "+action)
			return
		}
		target += "/" + action
	}
	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, r.Method, target, bytes.NewReader(raw))
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	request.Header.Set("Authorization", "Bearer "+ttsString(firstNonEmpty(connection["accessToken"], connection["apiKey"])))
	request.Header.Set("Accept", "application/json")
	if !polling {
		if contentType := r.Header.Get("Content-Type"); contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}
		if idempotency := r.Header.Get("idempotency-key"); idempotency != "" {
			request.Header.Set("Idempotency-Key", idempotency)
		}
	}
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		status := 502
		if ctx.Err() == context.DeadlineExceeded {
			status = 504
		}
		g.webError(w, status, err.Error())
		return
	}
	defer response.Body.Close()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.Header().Set("x-9router-connection-id", ttsString(connection["id"]))
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
}

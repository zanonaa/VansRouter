package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

var controlCharacters = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)

func (g *Gateway) webError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	nodeReply(w, status, map[string]any{"error": map[string]any{"message": message, "code": status}})
}

func (g *Gateway) resolveWeb(w http.ResponseWriter, r *http.Request, body map[string]any, kind string) (string, providers.WebConfig, bool) {
	if !g.key(r) {
		g.webError(w, 401, "API key required")
		return "", providers.WebConfig{}, false
	}
	input := ttsString(firstNonEmpty(body["provider"], body["model"]))
	if input == "" {
		g.webError(w, 400, "Missing required field: provider (or model)")
		return "", providers.WebConfig{}, false
	}
	id := providers.NormalizeID(input)
	config, ok := providers.WebConfigs()[id]
	selected := config.Search
	if kind == "fetch" {
		selected = config.Fetch
	}
	if !ok || selected == nil {
		g.webError(w, 400, "Provider "+id+" does not support web "+kind)
		return "", providers.WebConfig{}, false
	}
	return id, config, true
}

func sanitizeSearchQuery(query string) (string, string) {
	if controlCharacters.MatchString(query) {
		return "", "Query contains invalid control characters"
	}
	clean := strings.Join(strings.Fields(strings.TrimSpace(query)), " ")
	if clean == "" {
		return "", "Query is empty after normalization"
	}
	return clean, ""
}

func (g *Gateway) doJSON(ctx context.Context, method, target string, headers map[string]string, payload any, timeout time.Duration) (int, []byte, error) {
	var reader io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(encoded)
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, strings.TrimSpace(value))
	}
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return 504, nil, err
		}
		return 502, nil, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	return response.StatusCode, body, err
}

func (g *Gateway) Search(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		g.webError(w, 400, "Invalid JSON body")
		return
	}
	id, config, ok := g.resolveWeb(w, r, body, "search")
	if !ok {
		return
	}
	query, message := sanitizeSearchQuery(ttsString(body["query"]))
	if message != "" {
		g.webError(w, 400, message)
		return
	}
	base := strings.TrimRight(ttsString(config.Search["baseUrl"]), "/")
	token := ""
	if !config.NoAuth {
		connection, err := g.connection(r.Context(), id)
		if err != nil {
			g.webError(w, 400, "No credentials for provider: "+id)
			return
		}
		token = ttsString(firstNonEmpty(connection["apiKey"], connection["accessToken"]))
		body["_providerSpecificData"] = connection["providerSpecificData"]
		if token == "" {
			g.webError(w, 401, "No credentials for provider: "+id)
			return
		}
	}
	kind := ttsString(body["search_type"])
	if kind == "" {
		kind = "web"
	}
	requested := body["max_results"]
	if id == "exa" {
		options := exaOptions(body)
		if value, ok := options["numResults"]; ok {
			requested = value
		}
	}
	max := int(pxpipeNum(requested))
	if max <= 0 {
		max = int(pxpipeNum(config.Search["defaultMaxResults"]))
	}
	if max <= 0 {
		max = 5
	}
	limit := int(pxpipeNum(config.Search["maxMaxResults"]))
	if limit > 0 && max > limit {
		max = limit
	}
	method, target, headers, payload, err := buildBasicSearch(id, base, token, query, kind, max, body)
	if err != nil {
		g.webError(w, 501, err.Error())
		return
	}
	started := time.Now()
	status, upstream, err := g.doJSON(r.Context(), method, target, headers, payload, 15*time.Second)
	if err != nil {
		g.webError(w, status, id+" error: "+err.Error())
		return
	}
	if status < 200 || status >= 300 {
		g.webError(w, status, fmt.Sprintf("%s returned %d: %s", id, status, strings.TrimSpace(string(upstream))))
		return
	}
	var parsed map[string]any
	if json.Unmarshal(upstream, &parsed) != nil {
		g.webError(w, 502, id+" returned invalid JSON")
		return
	}
	results, total, metadata, pagination, err := normalizeSearch(id, kind, parsed, max, time.Now())
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	duration := time.Since(started).Milliseconds()
	cost := config.Search["costPerQuery"]
	if costs, ok := metadata["costDollars"].(map[string]any); ok {
		if reported, ok := costs["total"].(float64); ok {
			cost = reported
		}
	}
	response := map[string]any{"provider": id, "query": query, "results": results, "answer": nil, "usage": map[string]any{"queries_used": 1, "search_cost_usd": cost}, "metrics": map[string]any{"response_time_ms": duration, "upstream_latency_ms": duration, "total_results_available": total}, "errors": []any{}}
	if id == "exa" {
		response["exa"] = metadata
	}
	if pagination != nil {
		response["pagination"] = pagination
	}
	if credits := pxpipeNum(config.Search["creditsPerResult"]); credits > 0 {
		response["usage"].(map[string]any)["provider_credits_used"] = float64(len(results)) * credits
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	nodeReply(w, 200, response)
}

func (g *Gateway) Fetch(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		g.webError(w, 400, "Invalid JSON body")
		return
	}
	id, config, ok := g.resolveWeb(w, r, body, "fetch")
	if !ok {
		return
	}
	targetURL := ttsString(body["url"])
	if err := publicURL(targetURL); err != nil {
		g.webError(w, 400, err.Error())
		return
	}
	connection, err := g.connection(r.Context(), id)
	token := ""
	if err == nil {
		token = ttsString(firstNonEmpty(connection["apiKey"], connection["accessToken"]))
	}
	format := ttsString(body["format"])
	if format == "" {
		format = "markdown"
	}
	endpoint := strings.TrimRight(ttsString(config.Fetch["baseUrl"]), "/")
	payload := map[string]any{"url": targetURL}
	headers := map[string]string{"Content-Type": "application/json"}
	switch id {
	case "firecrawl":
		payload["formats"] = []any{format}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	case "tavily":
		payload = map[string]any{"urls": []any{targetURL}, "extract_depth": "basic"}
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	case "exa":
		payload = map[string]any{"ids": []any{targetURL}, "text": true}
		if token != "" {
			headers["x-api-key"] = token
		}
	case "tinyfish":
		payload = map[string]any{"urls": []any{targetURL}, "format": format}
		headers["X-API-Key"] = token
	case "jina-reader":
		endpoint += "/"
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}
	if id == "tinyfish" && format != "markdown" && format != "html" {
		g.webError(w, 400, "Unsupported TinyFish format: "+format)
		return
	}
	started := time.Now()
	status, upstream, err := g.doJSON(r.Context(), "POST", endpoint, headers, payload, 15*time.Second)
	if err != nil {
		g.webError(w, status, err.Error())
		return
	}
	if status < 200 || status >= 300 {
		g.webError(w, status, fmt.Sprintf("%s returned %d", id, status))
		return
	}
	normalized, err := normalizeFetch(id, targetURL, format, upstream, int(pxpipeNum(body["max_characters"])), config.Fetch["costPerQuery"], time.Since(started).Milliseconds())
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	nodeReply(w, 200, normalized)
}

func (g *Gateway) GeminiNative(w http.ResponseWriter, r *http.Request) {
	if !g.key(r) {
		g.webError(w, 401, "API key required")
		return
	}
	path := chi.URLParam(r, "path")
	action := ":generateContent"
	if strings.Contains(path, ":streamGenerateContent") {
		action = ":streamGenerateContent"
	}
	model := strings.TrimPrefix(strings.NewReplacer(":streamGenerateContent", "", ":generateContent", "").Replace(path), "models/")
	connection, err := g.connection(r.Context(), "gemini")
	if err != nil {
		g.webError(w, 400, "No credentials for provider: gemini")
		return
	}
	body, _ := io.ReadAll(r.Body)
	target := "https://generativelanguage.googleapis.com/v1beta/models/" + url.PathEscape(model) + action
	request, err := http.NewRequestWithContext(r.Context(), "POST", target, bytes.NewReader(body))
	if err != nil {
		g.webError(w, 500, err.Error())
		return
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("x-goog-api-key", ttsString(connection["apiKey"]))
	client := g.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		g.webError(w, 502, err.Error())
		return
	}
	defer response.Body.Close()
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", response.Header.Get("Content-Type"))
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
}

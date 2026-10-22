package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
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

func publicURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("Invalid URL format")
	}
	host := parsed.Hostname()
	if strings.EqualFold(host, "localhost") || strings.HasSuffix(strings.ToLower(host), ".local") {
		return fmt.Errorf("URL target is not public")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
		return fmt.Errorf("URL target is not public")
	}
	return nil
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
		if token == "" {
			g.webError(w, 401, "No credentials for provider: "+id)
			return
		}
	}
	method, target, payload := "POST", base, map[string]any{"query": query}
	headers := map[string]string{"Content-Type": "application/json", "Authorization": "Bearer " + token}
	switch id {
	case "serper":
		target += "/search"
		payload = map[string]any{"q": query}
		headers = map[string]string{"Content-Type": "application/json", "X-API-Key": token}
	case "brave-search":
		method = "GET"
		values := url.Values{"q": {query}}
		target += "/web/search?" + values.Encode()
		payload = nil
		headers = map[string]string{"Accept": "application/json", "X-Subscription-Token": token}
	case "tavily":
		payload["api_key"] = token
		delete(headers, "Authorization")
	case "searxng":
		method = "GET"
		values := url.Values{"q": {query}, "format": {"json"}}
		target += "?" + values.Encode()
		payload = nil
		headers = map[string]string{}
	}
	status, upstream, err := g.doJSON(r.Context(), method, target, headers, payload, 15*time.Second)
	if err != nil {
		g.webError(w, status, id+" error: "+err.Error())
		return
	}
	if status < 200 || status >= 300 {
		g.webError(w, status, fmt.Sprintf("%s returned %d: %s", id, status, strings.TrimSpace(string(upstream))))
		return
	}
	var parsed any
	if json.Unmarshal(upstream, &parsed) != nil {
		g.webError(w, 502, id+" returned invalid JSON")
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	nodeReply(w, 200, map[string]any{"provider": id, "query": query, "results": parsed})
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
		payload = map[string]any{"api_key": token, "urls": []any{targetURL}}
	case "exa":
		payload = map[string]any{"urls": []any{targetURL}}
		headers["Authorization"] = "Bearer " + token
	case "tinyfish":
		payload = map[string]any{"urls": []any{targetURL}, "format": format}
		headers["X-API-Key"] = token
	case "jina-reader":
		endpoint += "/" + targetURL
		payload = nil
		if token != "" {
			headers["Authorization"] = "Bearer " + token
		}
	}
	method := "POST"
	if id == "jina-reader" {
		method = "GET"
	}
	status, upstream, err := g.doJSON(r.Context(), method, endpoint, headers, payload, 15*time.Second)
	if err != nil {
		g.webError(w, status, err.Error())
		return
	}
	if status < 200 || status >= 300 {
		g.webError(w, status, fmt.Sprintf("%s returned %d", id, status))
		return
	}
	text := string(upstream)
	var parsed any
	if json.Unmarshal(upstream, &parsed) == nil {
		text = ttsString(parsed)
		if text == "" {
			text = string(upstream)
		}
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	nodeReply(w, 200, map[string]any{"provider": id, "url": targetURL, "content": map[string]any{"format": format, "text": text, "length": len([]rune(text))}})
}

func (g *Gateway) Speech(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		g.webError(w, 400, "Invalid JSON body")
		return
	}
	if !g.key(r) {
		g.webError(w, 401, "API key required")
		return
	}
	provider, model := splitTranslatorModel(ttsString(body["model"]))
	if provider != "openai" || ttsString(body["input"]) == "" {
		g.webError(w, 400, "Missing OpenAI model or input")
		return
	}
	connection, err := g.connection(r.Context(), provider)
	if err != nil {
		g.webError(w, 400, "No credentials for provider: openai")
		return
	}
	voice := "alloy"
	if strings.Contains(model, "/") {
		parts := strings.SplitN(model, "/", 2)
		model, voice = parts[0], parts[1]
	}
	status, upstream, err := g.doJSON(r.Context(), "POST", "https://api.openai.com/v1/audio/speech", map[string]string{"Authorization": "Bearer " + ttsString(connection["apiKey"]), "Content-Type": "application/json"}, map[string]any{"model": model, "voice": voice, "input": body["input"]}, 60*time.Second)
	if err != nil {
		g.webError(w, status, err.Error())
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "audio/mpeg")
	w.WriteHeader(status)
	w.Write(upstream)
}

func (g *Gateway) Transcriptions(w http.ResponseWriter, r *http.Request) {
	if !g.key(r) {
		g.webError(w, 401, "API key required")
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		g.webError(w, 400, "Invalid multipart form data")
		return
	}
	provider, model := splitTranslatorModel(r.FormValue("model"))
	file, header, err := r.FormFile("file")
	if provider != "openai" || model == "" || err != nil {
		g.webError(w, 400, "Missing OpenAI model or file")
		return
	}
	defer file.Close()
	connection, connErr := g.connection(r.Context(), provider)
	if connErr != nil {
		g.webError(w, 400, "No credentials for provider: openai")
		return
	}
	var payload bytes.Buffer
	form := multipart.NewWriter(&payload)
	part, _ := form.CreateFormFile("file", header.Filename)
	io.Copy(part, file)
	form.WriteField("model", model)
	form.Close()
	request, err := http.NewRequestWithContext(r.Context(), "POST", "https://api.openai.com/v1/audio/transcriptions", &payload)
	request.Header.Set("Authorization", "Bearer "+ttsString(connection["apiKey"]))
	request.Header.Set("Content-Type", form.FormDataContentType())
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
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

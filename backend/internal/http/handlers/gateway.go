package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type Gateway struct {
	DB                *sql.DB
	Client            *http.Client
	Send              *TranslatorSend
	googleSpeechMu    sync.Mutex
	googleSpeechCache googleSpeechToken
	googleSpeechIndex uint64
	edgeSpeechMu      sync.Mutex
	edgeSpeechCache   edgeSpeechToken
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
	active := true
	connections, err := (database.Connections{DB: g.DB}).List(ctx, provider, &active)
	if err != nil {
		return nil, err
	}
	if len(connections) == 0 {
		return nil, sql.ErrNoRows
	}
	return connections[0], nil
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
	g.catalogModels(w, r)
}
func (g *Gateway) OpenAIChat(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	key, ok := g.modelAuth(w, r)
	if !ok {
		return
	}
	if !gatewayPermission(key, "allowedKinds", "llm") {
		g.webError(w, 403, "Chat/LLM requests are not allowed for this API key")
		return
	}
	if g.Send == nil {
		g.NotPorted(w, r)
		return
	}
	body, err := poolInput(r)
	if err != nil {
		nodeReply(w, 400, map[string]any{"error": "Invalid JSON body"})
		return
	}
	// O /v1/models publica combo como "combo/<nome>" (paridade com a origem);
	// o prefixo é só de listagem e precisa sair antes de resolver o combo —
	// senão o provider "combo" não existe e a chamada falha com 404.
	if model := ttsString(body["model"]); strings.HasPrefix(model, "combo/") {
		body["model"] = strings.TrimPrefix(model, "combo/")
	}
	if !strings.Contains(ttsString(body["model"]), "/") {
		if g.chatCombo(w, r, key, body) {
			return
		}
	}
	modelID := ttsString(body["model"])
	provider, model := splitTranslatorModel(modelID)
	prefix := ""
	if parts := strings.SplitN(modelID, "/", 2); len(parts) == 2 {
		prefix = parts[0]
	}
	// Prefixos de provider-node (ex.: "da") são definidos pelo usuário e não
	// existem no registry; a origem resolve o node antes de procurar conexão
	// (src/sse/services/model.js getModelInfo). Sem isso o gateway procurava a
	// conexão pelo prefixo e devolvia "No active connection for provider: da".
	if provider != "" && prefix != "" {
		if nodeID := g.providerNodeID(r.Context(), prefix); nodeID != "" {
			provider = nodeID
		}
	}
	if provider != "" && !g.providerAllowed(key, provider, prefix) {
		g.webError(w, 403, `Provider "`+provider+`" is not allowed for this API key`)
		return
	}
	source := detectRequestFormat(body)
	if strings.Contains(r.URL.Path, "/v1/messages") {
		source = "claude"
	}
	if strings.Contains(r.URL.Path, "/v1/responses") {
		source = "openai-responses"
	}
	// Default do OpenAI: sem "stream" explícito, resposta JSON. O bug anterior
	// (stream=true quando omitido) fazia o gateway devolver SSE a clientes
	// que pediram JSON — achado no teste real dos combos.
	stream := body["stream"] == true
	format := providers.TargetFormat(provider)
	// grok-cli fala Responses API com stream forçado e fingerprint próprio;
	// o executor dedicado cobre request, headers e fold para clientes JSON.
	if provider == "grok-cli" && format == "openai-responses" {
		g.Send.SendGrokCLI(w, r, provider, model, body, source, stream)
		return
	}
	if provider == "" || format != "openai" && format != "claude" && format != "gemini" {
		g.NotPorted(w, r)
		return
	}
	if source != format {
		if stream && !(source == "openai" && (format == "claude" || format == "gemini") || (source == "claude" || source == "openai-responses") && format == "openai") {
			g.webError(w, 501, "Streaming conversion not implemented for this format pair")
			return
		}
		intermediate, err := translateSource(body, source)
		if err != nil {
			g.webError(w, 501, err.Error())
			return
		}
		body, err = translateTarget(intermediate, format, model)
		if err != nil {
			g.webError(w, 501, err.Error())
			return
		}
	}
	body["stream"] = stream
	encoded, _ := json.Marshal(map[string]any{"provider": provider, "model": model, "body": body})
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(bytes.NewReader(encoded))
	if source != format {
		if stream {
			converted := &streamConversion{destination: w, source: source, target: format, model: model, textBlock: -1, serverTool: -1}
			g.Send.Send(converted, clone)
			converted.finish()
			return
		}
		converted := &convertedResponse{destination: w, source: source, target: format, model: model}
		g.Send.Send(converted, clone)
		converted.finish()
		return
	}
	g.Send.Send(w, clone)
}
func filterToOpenAI(body map[string]any) map[string]any {
	messages, ok := body["messages"].([]any)
	if !ok {
		return body
	}
	filtered := []any{}
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if message["role"] == "developer" {
			message["role"] = "system"
		}
		if content, ok := message["content"].([]any); ok {
			blocks := []any{}
			for _, rawBlock := range content {
				block, ok := rawBlock.(map[string]any)
				if !ok || block["type"] == "thinking" || block["type"] == "redacted_thinking" {
					continue
				}
				if block["type"] == "text" || block["type"] == "image_url" {
					delete(block, "signature")
					delete(block, "cache_control")
					blocks = append(blocks, block)
				}
			}
			if len(blocks) == 0 {
				blocks = append(blocks, map[string]any{"type": "text", "text": ""})
			}
			message["content"] = blocks
		}
		filtered = append(filtered, message)
	}
	body["messages"] = filtered
	return body
}

func (g *Gateway) Responses(w http.ResponseWriter, r *http.Request) {
	body, err := poolInput(r)
	if err != nil {
		nodeReply(w, 400, map[string]any{"error": "Invalid JSON body"})
		return
	}
	if _, exists := body["stream"]; !exists {
		body["stream"] = false
	}
	encoded, _ := json.Marshal(body)
	clone := r.Clone(r.Context())
	clone.Body = io.NopCloser(bytes.NewReader(encoded))
	g.OpenAIChat(w, clone)
}

func (g *Gateway) NotPorted(w http.ResponseWriter, r *http.Request) {
	if !g.key(r) {
		nodeReply(w, 401, map[string]any{"error": "API key required"})
		return
	}
	nodeReply(w, 501, map[string]any{"error": "Gateway executor not ported", "path": r.URL.Path})
}

// Gateway model listing is implemented in gateway_models.go.

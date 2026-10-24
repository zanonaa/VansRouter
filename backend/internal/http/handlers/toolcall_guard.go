package handlers

// Tool Call Guard: alguns upstreams (GLM-5.x, GPT-5.x/Codex, Grok) fazem
// decoding do schema de tools sempre preenchendo TODAS as propriedades, mesmo
// as opcionais. Em vez de omitir, inventam valores (sessionID "ses_invalid",
// model "auto"/""), o que quebra loops de tool (ex.: "Subagent session not
// found" no OpenCode) e nunca converge.
//
// O schema repassado ao upstream é o do cliente e está correto (required só com
// os campos obrigatórios); o defeito é do modelo. Este guard corrige o efeito
// de forma determinística: propriedades opcionais que são referências (IDs,
// sessões, model) só são aceitas quando o valor aparece na própria conversa.
// Valores inventados são removidos da resposta antes de chegar ao cliente.

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
)

const toolGuardHeader = "x-9router-tool-guard"

type toolGuardMode int

const (
	toolGuardUnknown toolGuardMode = iota
	toolGuardPass
	toolGuardSSE
	toolGuardJSON
)

type toolCallKey struct {
	choice int
	tool   int
}

type toolCallPending struct {
	name string
	args strings.Builder
}

// toolCallGuard envolve o ResponseWriter para reescrever, no stream, os
// argumentos de tool calls. Eventos de texto passam sem atraso; apenas os
// argumentos de tool call são retidos até o fim da chamada, quando o JSON
// completo pode ser validado.
type toolCallGuard struct {
	http.ResponseWriter
	flusher http.Flusher

	tools  map[string]map[string]any
	corpus string
	// Subagent continuation IDs must come from successful tool results, not
	// assistant guesses, errors, quoted examples, or the parent session ID.
	subagentSessions map[string]bool

	mode          toolGuardMode
	headerWritten bool
	buf           []byte
	pending       map[toolCallKey]*toolCallPending

	baseID      string
	baseModel   string
	baseCreated int64
}

func newToolCallGuard(w http.ResponseWriter, r *http.Request, body map[string]any) *toolCallGuard {
	g := &toolCallGuard{ResponseWriter: w, pending: map[toolCallKey]*toolCallPending{}}
	if f, ok := w.(http.Flusher); ok {
		g.flusher = f
	}
	if strings.EqualFold(os.Getenv("VANSROUTER_TOOL_GUARD"), "off") {
		g.mode = toolGuardPass
		return g
	}
	if r != nil && strings.EqualFold(r.Header.Get(toolGuardHeader), "off") {
		g.mode = toolGuardPass
		return g
	}
	g.tools = toolGuardSchemas(body["tools"])
	if !toolGuardNeeded(g.tools) {
		g.mode = toolGuardPass
		return g
	}
	g.corpus = toolGuardCorpus(body)
	g.subagentSessions = toolGuardSubagentSessions(body)
	return g
}

var toolGuardSessionMarker = regexp.MustCompile(`sessionID(?:"\s*:\s*"|\s*[:=]\s*"?)?\s*(ses_[A-Za-z0-9]+)`)

func toolGuardSubagentSessions(body map[string]any) map[string]bool {
	valid := map[string]bool{}
	messages, _ := body["messages"].([]any)
	for _, raw := range messages {
		message, _ := raw.(map[string]any)
		if message["role"] != "tool" {
			continue
		}
		content, _ := json.Marshal(message["content"])
		text := strings.ReplaceAll(string(content), `\"`, `"`)
		// Failed calls echo the guessed ID and must never authorize it.
		if strings.Contains(text, "Subagent session not found") || strings.Contains(text, `state=\"error`) || strings.Contains(text, "Subagent failed") {
			continue
		}
		for _, match := range toolGuardSessionMarker.FindAllStringSubmatch(text, -1) {
			valid[match[1]] = true
		}
	}
	return valid
}

func toolGuardSchemas(raw any) map[string]map[string]any {
	out := map[string]map[string]any{}
	tools, _ := raw.([]any)
	for _, item := range tools {
		tool, _ := item.(map[string]any)
		if tool == nil {
			continue
		}
		name := ""
		var parameters map[string]any
		if function, ok := tool["function"].(map[string]any); ok {
			name = ttsString(function["name"])
			parameters, _ = function["parameters"].(map[string]any)
		}
		if name == "" {
			name = ttsString(tool["name"])
		}
		if parameters == nil {
			parameters, _ = tool["parameters"].(map[string]any)
		}
		if name != "" && parameters != nil {
			out[name] = parameters
		}
	}
	return out
}

func toolGuardNeeded(tools map[string]map[string]any) bool {
	for _, schema := range tools {
		properties, _ := schema["properties"].(map[string]any)
		required := toolGuardRequired(schema)
		for name, raw := range properties {
			if required[name] {
				continue
			}
			prop, _ := raw.(map[string]any)
			if toolGuardReference(name, prop) {
				return true
			}
		}
	}
	return false
}

func toolGuardCorpus(body map[string]any) string {
	// tools ficam de fora: descrições/exemplos do schema não são evidência de
	// que um ID exista na conversa.
	without := make(map[string]any, len(body))
	for key, value := range body {
		if key == "tools" {
			continue
		}
		without[key] = value
	}
	encoded, err := json.Marshal(without)
	if err != nil {
		return ""
	}
	return strings.ToLower(string(encoded))
}

func toolGuardRequired(schema map[string]any) map[string]bool {
	out := map[string]bool{}
	required, _ := schema["required"].([]any)
	for _, item := range required {
		if name := ttsString(item); name != "" {
			out[name] = true
		}
	}
	return out
}

// toolGuardReference reconhece parâmetros opcionais que apontam para algo que
// precisa existir: IDs (camelCase/ID/UUID ou com pattern) e model.
func toolGuardReference(name string, prop map[string]any) bool {
	if prop != nil {
		if _, ok := prop["pattern"]; ok {
			return true
		}
	}
	lower := strings.ToLower(name)
	switch lower {
	case "model", "id", "uuid", "guid", "key", "token":
		return true
	}
	if strings.HasSuffix(name, "ID") || strings.HasSuffix(name, "Id") ||
		strings.HasSuffix(name, "UUID") || strings.HasSuffix(name, "Uuid") {
		return true
	}
	for _, suffix := range []string{"_id", "-id", ".id", "_uuid", "_guid"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func (g *toolCallGuard) ensureMode() {
	if g.mode != toolGuardUnknown {
		return
	}
	contentType := strings.ToLower(g.Header().Get("Content-Type"))
	switch {
	case strings.Contains(contentType, "event-stream") || strings.Contains(contentType, "ndjson"):
		g.mode = toolGuardSSE
	case strings.Contains(contentType, "json"):
		g.mode = toolGuardJSON
	default:
		g.mode = toolGuardPass
	}
}

func (g *toolCallGuard) WriteHeader(status int) {
	if g.headerWritten {
		return
	}
	g.headerWritten = true
	g.ensureMode()
	g.ResponseWriter.WriteHeader(status)
}

func (g *toolCallGuard) Write(p []byte) (int, error) {
	if !g.headerWritten {
		g.WriteHeader(http.StatusOK)
	}
	switch g.mode {
	case toolGuardSSE:
		g.buf = append(g.buf, p...)
		g.drainSSE()
		return len(p), nil
	case toolGuardJSON:
		g.buf = append(g.buf, p...)
		return len(p), nil
	default:
		return g.ResponseWriter.Write(p)
	}
}

func (g *toolCallGuard) Flush() {
	if g.mode == toolGuardSSE {
		g.drainSSE()
	}
	if g.flusher != nil {
		g.flusher.Flush()
	}
}

// Close drena o que restou e libera respostas JSON buffers.
func (g *toolCallGuard) Close() {
	switch g.mode {
	case toolGuardSSE:
		g.flushPending()
		if len(g.buf) > 0 {
			g.ResponseWriter.Write(g.buf)
			g.buf = nil
		}
	case toolGuardJSON:
		out := g.sanitizeJSON(g.buf)
		if len(out) > 0 {
			g.ResponseWriter.Write(out)
		}
		g.buf = nil
	}
	if g.flusher != nil {
		g.flusher.Flush()
	}
}

func toolGuardNextEvent(buf []byte) (event, rest []byte, separator string, ok bool) {
	lf := bytes.Index(buf, []byte("\n\n"))
	crlf := bytes.Index(buf, []byte("\r\n\r\n"))
	switch {
	case lf < 0 && crlf < 0:
		return nil, buf, "", false
	case crlf >= 0 && (lf < 0 || crlf < lf):
		return buf[:crlf+4], buf[crlf+4:], "\r\n\r\n", true
	default:
		return buf[:lf+2], buf[lf+2:], "\n\n", true
	}
}

func (g *toolCallGuard) drainSSE() {
	for {
		event, rest, separator, ok := toolGuardNextEvent(g.buf)
		if !ok {
			return
		}
		g.buf = rest
		g.processSSE(event, separator)
	}
}

func (g *toolCallGuard) processSSE(event []byte, separator string) {
	if !bytes.Contains(event, []byte("data:")) {
		g.ResponseWriter.Write(event)
		return
	}
	// Formatos Responses/Claude usam "event:" e não são chat completions.
	if bytes.Contains(event, []byte("\nevent:")) || bytes.HasPrefix(bytes.TrimSpace(event), []byte("event:")) {
		g.ResponseWriter.Write(event)
		return
	}
	payload := toolGuardDataPayload(event)
	if payload == "" {
		g.ResponseWriter.Write(event)
		return
	}
	if strings.TrimSpace(payload) == "[DONE]" {
		g.flushPending()
		g.ResponseWriter.Write(event)
		return
	}
	var chunk map[string]any
	if json.Unmarshal([]byte(payload), &chunk) != nil {
		g.ResponseWriter.Write(event)
		return
	}
	choices, _ := chunk["choices"].([]any)
	if len(choices) == 0 {
		g.ResponseWriter.Write(event)
		return
	}
	g.captureBase(chunk)
	if toolGuardHasFinish(choices) {
		g.flushPending()
	}
	changed, emit := g.consumeChatChunk(choices)
	if !emit {
		return
	}
	if !changed {
		g.ResponseWriter.Write(event)
		return
	}
	encoded, err := json.Marshal(chunk)
	if err != nil {
		g.ResponseWriter.Write(event)
		return
	}
	g.ResponseWriter.Write([]byte("data: " + string(encoded) + separator))
}

func toolGuardDataPayload(event []byte) string {
	lines := strings.Split(string(event), "\n")
	parts := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, "data:") {
			parts = append(parts, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	return strings.Join(parts, "\n")
}

func toolGuardHasFinish(choices []any) bool {
	for _, raw := range choices {
		choice, _ := raw.(map[string]any)
		if choice == nil {
			continue
		}
		if finish, ok := choice["finish_reason"]; ok && finish != nil {
			return true
		}
	}
	return false
}

func (g *toolCallGuard) captureBase(chunk map[string]any) {
	if g.baseID == "" {
		g.baseID = ttsString(chunk["id"])
	}
	if g.baseModel == "" {
		g.baseModel = ttsString(chunk["model"])
	}
	if g.baseCreated == 0 {
		if created, ok := chunk["created"].(float64); ok {
			g.baseCreated = int64(created)
		}
	}
}

func toolGuardIndex(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	}
	return 0
}

// consumeChatChunk retira os argumentos de tool call do evento (serão emitidos
// completos no fim da chamada) e informa se algo mudou / se o evento ainda tem
// conteúdo útil para o cliente.
func (g *toolCallGuard) consumeChatChunk(choices []any) (bool, bool) {
	changed := false
	remain := false
	for _, raw := range choices {
		choice, _ := raw.(map[string]any)
		if choice == nil {
			continue
		}
		choiceIndex := toolGuardIndex(choice["index"])
		if _, ok := choice["finish_reason"]; ok {
			remain = true
		}
		delta, _ := choice["delta"].(map[string]any)
		if delta == nil {
			continue
		}
		calls, _ := delta["tool_calls"].([]any)
		if len(calls) > 0 {
			kept := make([]any, 0, len(calls))
			for _, rawCall := range calls {
				call, _ := rawCall.(map[string]any)
				if call == nil {
					continue
				}
				toolIndex := toolGuardIndex(call["index"])
				key := toolCallKey{choice: choiceIndex, tool: toolIndex}
				function, _ := call["function"].(map[string]any)
				if function != nil {
					state := g.pending[key]
					if state == nil {
						state = &toolCallPending{}
						g.pending[key] = state
					}
					if name := ttsString(function["name"]); name != "" {
						state.name = name
					}
					if args, ok := function["arguments"].(string); ok && args != "" {
						state.args.WriteString(args)
						delete(function, "arguments")
						changed = true
					}
					if len(function) == 0 {
						delete(call, "function")
					}
				}
				// Mantém id/type/name; descarta pedaços que só carregavam args.
				if len(call) == 1 {
					if _, onlyIndex := call["index"]; onlyIndex {
						continue
					}
				}
				if len(call) == 0 {
					continue
				}
				kept = append(kept, call)
			}
			if len(kept) == 0 {
				delete(delta, "tool_calls")
			} else {
				delta["tool_calls"] = kept
			}
			if len(delta) == 0 {
				delete(choice, "delta")
			}
		}
		if len(delta) > 0 {
			remain = true
		}
		if len(choice) > 1 {
			remain = true
		}
	}
	return changed, remain
}

func (g *toolCallGuard) flushPending() {
	if len(g.pending) == 0 {
		return
	}
	byChoice := map[int][]any{}
	for key, state := range g.pending {
		raw := state.args.String()
		if state.name == "" && raw == "" {
			continue
		}
		sanitized, dropped := g.sanitizeArguments(state.name, raw)
		if len(dropped) > 0 {
			slog.Info("TOOL_GUARD dropped ungrounded optional tool arguments",
				"tool", state.name, "fields", strings.Join(dropped, ","))
		}
		call := map[string]any{"index": key.tool, "function": map[string]any{"arguments": sanitized}}
		byChoice[key.choice] = append(byChoice[key.choice], call)
	}
	g.pending = map[toolCallKey]*toolCallPending{}
	if len(byChoice) == 0 {
		return
	}
	indexes := make([]int, 0, len(byChoice))
	for index := range byChoice {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	choices := make([]any, 0, len(indexes))
	for _, index := range indexes {
		calls := byChoice[index]
		sort.Slice(calls, func(i, j int) bool {
			return toolGuardIndex(calls[i].(map[string]any)["index"]) < toolGuardIndex(calls[j].(map[string]any)["index"])
		})
		choices = append(choices, map[string]any{
			"index":         index,
			"delta":         map[string]any{"tool_calls": calls},
			"finish_reason": nil,
		})
	}
	event := map[string]any{
		"id":      g.baseID,
		"object":  "chat.completion.chunk",
		"created": g.baseCreated,
		"model":   g.baseModel,
		"choices": choices,
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return
	}
	g.ResponseWriter.Write([]byte("data: " + string(encoded) + "\n\n"))
}

func (g *toolCallGuard) sanitizeJSON(raw []byte) []byte {
	var body map[string]any
	if json.Unmarshal(raw, &body) != nil {
		return raw
	}
	choices, _ := body["choices"].([]any)
	changed := false
	for _, rawChoice := range choices {
		choice, _ := rawChoice.(map[string]any)
		message, _ := choice["message"].(map[string]any)
		calls, _ := message["tool_calls"].([]any)
		for _, rawCall := range calls {
			call, _ := rawCall.(map[string]any)
			function, _ := call["function"].(map[string]any)
			args, ok := function["arguments"].(string)
			if !ok {
				continue
			}
			sanitized, dropped := g.sanitizeArguments(ttsString(function["name"]), args)
			if len(dropped) > 0 {
				slog.Info("TOOL_GUARD dropped ungrounded optional tool arguments",
					"tool", ttsString(function["name"]), "fields", strings.Join(dropped, ","))
				function["arguments"] = sanitized
				changed = true
			}
		}
	}
	if !changed {
		return raw
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return raw
	}
	return encoded
}

// sanitizeArguments remove propriedades opcionais inventadas. Só mexe em
// referências: IDs com pattern/sufixo de ID e "model".
func (g *toolCallGuard) sanitizeArguments(name, raw string) (string, []string) {
	schema := g.tools[name]
	if schema == nil || raw == "" {
		return raw, nil
	}
	var args map[string]any
	if json.Unmarshal([]byte(raw), &args) != nil {
		return raw, nil
	}
	properties, _ := schema["properties"].(map[string]any)
	required := toolGuardRequired(schema)
	dropped := []string{}
	for key, value := range args {
		if required[key] {
			continue
		}
		prop, _ := properties[key].(map[string]any)
		if value == nil {
			delete(args, key)
			dropped = append(dropped, key)
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		if strings.TrimSpace(text) == "" {
			delete(args, key)
			dropped = append(dropped, key)
			continue
		}
		if !toolGuardReference(key, prop) {
			continue
		}
		if name == "subagent" && key == "sessionID" && !g.subagentSessions[text] {
			delete(args, key)
			dropped = append(dropped, key)
			continue
		}
		if strings.EqualFold(key, "model") && !strings.Contains(text, "/") {
			// O cliente exige "providerID/modelID"; sem barra é placeholder.
			delete(args, key)
			dropped = append(dropped, key)
			continue
		}
		if len(text) < 3 || !g.grounded(text) {
			delete(args, key)
			dropped = append(dropped, key)
		}
	}
	if len(dropped) == 0 {
		return raw, nil
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return raw, nil
	}
	return string(encoded), dropped
}

func (g *toolCallGuard) grounded(value string) bool {
	if g.corpus == "" {
		return true
	}
	return strings.Contains(g.corpus, strings.ToLower(value))
}

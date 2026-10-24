package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type usageContextKey struct{}
type requestUsage struct {
	provider, model, connection string
	tokens                      map[string]any
	live                        *UsageLive
	// inputEstimate e contentChars alimentam o fallback de estimativa quando o
	// upstream não devolve usage (paridade com estimateUsage da origem Node).
	// Sem isso o request some do dashboard: o registro só acontece quando há
	// prompt_tokens/completion_tokens > 0.
	inputEstimate int
	contentChars  int
	succeeded     bool
}

// Mesmo buffer aplicado por addBufferToUsage na origem Node ao estimar usage.
const usageEstimateBuffer = 2000

func estimateInputTokens(body map[string]any) int {
	if len(body) == 0 {
		return 0
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		return 0
	}
	return (len(encoded) + 3) / 4
}

func estimateOutputTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	if chars/4 < 1 {
		return 1
	}
	return chars / 4
}

func usageSelected(ctx context.Context, connection map[string]any) {
	if u, ok := ctx.Value(usageContextKey{}).(*requestUsage); ok {
		id := ttsString(connection["id"])
		if u.connection == id {
			return
		}
		if u.live != nil {
			u.live.Track(u.model, u.provider, u.connection, false, false)
		}
		u.connection = id
		if u.live != nil {
			u.live.Track(u.model, u.provider, id, true, false)
		}
	}
}

// Observe the upstream rather than the client response: translations must not
// discard accounting metadata, nor require buffering the entire stream.
type usageTransport struct {
	base  http.RoundTripper
	usage *requestUsage
}

func (t usageTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	response, err := t.base.RoundTrip(r)
	if err == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
		t.usage.succeeded = true
		response.Body = &usageBody{ReadCloser: response.Body, u: t.usage, stream: strings.Contains(response.Header.Get("Content-Type"), "event-stream") || strings.Contains(response.Header.Get("Content-Type"), "ndjson") || t.usage.provider == "commandcode"}
	}
	return response, err
}

type usageBody struct {
	io.ReadCloser
	u       *requestUsage
	pending []byte
	stream  bool
}

func (b *usageBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.pending = append(b.pending, p[:n]...)
	// Some Codex/Grok upstreams omit Content-Type and begin SSE with event:
	// rather than data:. Detect both (including a prefix split across reads).
	trimmed := bytes.TrimSpace(b.pending)
	if !b.stream && (bytes.HasPrefix(trimmed, []byte("data:")) || bytes.HasPrefix(trimmed, []byte("event:")) || bytes.HasPrefix(trimmed, []byte(":"))) {
		b.stream = true
	}
	for b.stream {
		index := bytes.IndexByte(b.pending, '\n')
		if index < 0 {
			break
		}
		line := bytes.TrimSpace(b.pending[:index])
		b.pending = b.pending[index+1:]
		if bytes.HasPrefix(line, []byte("data:")) {
			b.stream = true
			line = bytes.TrimSpace(bytes.TrimPrefix(line, []byte("data:")))
		}
		b.parse(line)
	}
	if len(b.pending) > 8<<20 {
		b.pending = nil
	}
	if err != nil {
		b.parse(bytes.TrimSpace(b.pending))
		b.pending = nil
	}
	return n, err
}
func (b *usageBody) parse(raw []byte) {
	var event map[string]any
	if json.Unmarshal(raw, &event) != nil {
		return
	}
	b.u.observe(event)
	if b.stream {
		b.u.countStreamContent(event)
	} else {
		b.u.countResponseContent(event)
	}
}

func (u *requestUsage) addText(value any) {
	switch typed := value.(type) {
	case string:
		u.contentChars += utf8.RuneCountInString(typed)
	case []any:
		for _, item := range typed {
			u.addText(item)
		}
	case map[string]any:
		u.addText(typed["text"])
		u.addText(typed["thinking"])
		u.addText(typed["content"])
	}
}

func (u *requestUsage) addToolCalls(value any) {
	calls, _ := value.([]any)
	for _, raw := range calls {
		call, _ := raw.(map[string]any)
		function, _ := call["function"].(map[string]any)
		u.addText(function["name"])
		u.addText(function["arguments"])
	}
}

func (u *requestUsage) addCandidates(value any) {
	candidates, _ := value.([]any)
	for _, raw := range candidates {
		candidate, _ := raw.(map[string]any)
		content, _ := candidate["content"].(map[string]any)
		u.addText(content["parts"])
	}
}

func (u *requestUsage) addResponsesOutput(value any) {
	items, _ := value.([]any)
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		u.addText(item["arguments"])
		u.addText(item["content"])
	}
}

func (u *requestUsage) addOpenAIChoices(value any, stream bool) {
	choices, _ := value.([]any)
	for _, raw := range choices {
		choice, _ := raw.(map[string]any)
		u.addText(choice["text"])
		if stream {
			delta, _ := choice["delta"].(map[string]any)
			u.addText(delta["content"])
			u.addText(delta["reasoning_content"])
			u.addText(delta["reasoning"])
			u.addToolCalls(delta["tool_calls"])
			continue
		}
		message, _ := choice["message"].(map[string]any)
		u.addText(message["content"])
		u.addText(message["reasoning_content"])
		u.addText(message["reasoning"])
		u.addToolCalls(message["tool_calls"])
	}
}

// countStreamContent soma apenas os deltas do stream. O evento terminal do
// Responses API repete o output completo, então ele só é usado quando nenhum
// delta chegou (executores que emitem apenas o evento final).
func (u *requestUsage) countStreamContent(event map[string]any) {
	eventType := ttsString(event["type"])
	u.addOpenAIChoices(event["choices"], true)
	if delta, ok := event["delta"].(map[string]any); ok {
		u.addText(delta["text"])
		u.addText(delta["thinking"])
		u.addText(delta["partial_json"])
	} else if text, ok := event["delta"].(string); ok && strings.Contains(eventType, "delta") {
		u.addText(text)
	}
	u.addText(event["arguments"])
	u.addCandidates(event["candidates"])
	if response, ok := event["response"].(map[string]any); ok {
		u.addCandidates(response["candidates"])
		if u.contentChars == 0 {
			u.addResponsesOutput(response["output"])
		}
	}
	if u.contentChars == 0 && (strings.HasSuffix(eventType, ".completed") || strings.HasSuffix(eventType, ".done")) {
		u.addResponsesOutput(event["output"])
	}
}

func (u *requestUsage) countResponseContent(event map[string]any) {
	u.addOpenAIChoices(event["choices"], false)
	// Claude: content é uma lista de blocos {type,text,thinking}.
	u.addText(event["content"])
	u.addCandidates(event["candidates"])
	u.addResponsesOutput(event["output"])
	if message, ok := event["message"].(map[string]any); ok {
		u.addText(message["content"])
	}
	if response, ok := event["response"].(map[string]any); ok {
		u.addText(response["content"])
		u.addCandidates(response["candidates"])
		u.addResponsesOutput(response["output"])
	}
}

// applyEstimatedUsage reproduz o fallback da origem: quando o provider não
// devolve usage, estima tokens a partir do corpo da requisição e do conteúdo
// recebido. Sem isso o request é servido com 200 mas nunca entra no histórico.
func (u *requestUsage) applyEstimatedUsage() {
	if !u.succeeded {
		return
	}
	prompt := pxpipeNum(u.tokens["prompt_tokens"])
	completion := pxpipeNum(u.tokens["completion_tokens"])
	if prompt == 0 && completion == 0 {
		// Sem conteúdo não há como estimar a saída; a origem também não
		// registra nada nesse caso (evita contar tentativas vazias/erros).
		if u.contentChars == 0 {
			return
		}
		if u.tokens == nil {
			u.tokens = map[string]any{}
		}
		u.tokens["prompt_tokens"] = float64(u.inputEstimate + usageEstimateBuffer)
		u.tokens["completion_tokens"] = float64(estimateOutputTokens(u.contentChars))
		u.tokens["estimated"] = true
		return
	}
	if completion == 0 && u.contentChars > 0 {
		u.tokens["completion_tokens"] = float64(estimateOutputTokens(u.contentChars))
		u.tokens["estimated"] = true
	}
	if prompt == 0 && u.inputEstimate > 0 {
		u.tokens["prompt_tokens"] = float64(u.inputEstimate)
		u.tokens["estimated"] = true
	}
}
func (u *requestUsage) observe(event map[string]any) {
	for _, key := range []string{"response", "message"} {
		if nested, ok := event[key].(map[string]any); ok {
			u.observe(nested)
		}
	}
	var raw map[string]any
	for _, key := range []string{"usage", "usageMetadata", "totalUsage"} {
		if value, ok := event[key].(map[string]any); ok {
			raw = value
		}
	}
	if event["prompt_eval_count"] != nil {
		raw = map[string]any{"prompt_tokens": event["prompt_eval_count"], "completion_tokens": event["eval_count"]}
	}
	if raw == nil {
		return
	}
	if u.tokens == nil {
		u.tokens = map[string]any{}
	}
	pairs := map[string][]string{"prompt_tokens": {"prompt_tokens", "input_tokens", "promptTokenCount", "inputTokens", "promptTokens"}, "completion_tokens": {"completion_tokens", "output_tokens", "candidatesTokenCount", "outputTokens", "completionTokens"}, "cached_tokens": {"cached_tokens", "cache_read_input_tokens", "cachedContentTokenCount"}, "cache_creation_input_tokens": {"cache_creation_input_tokens"}, "reasoning_tokens": {"reasoning_tokens"}}
	for canonical, keys := range pairs {
		for _, key := range keys {
			if value, ok := raw[key]; ok {
				u.tokens[canonical] = value
				break
			}
		}
	}
	for _, key := range []string{"prompt_tokens_details", "input_tokens_details"} {
		if details, ok := raw[key].(map[string]any); ok {
			if details["cached_tokens"] != nil {
				u.tokens["cached_tokens"] = details["cached_tokens"]
			}
			if details["cache_creation_tokens"] != nil {
				u.tokens["cache_creation_input_tokens"] = details["cache_creation_tokens"]
			}
		}
	}
	for _, key := range []string{"completion_tokens_details", "output_tokens_details"} {
		if details, ok := raw[key].(map[string]any); ok && details["reasoning_tokens"] != nil {
			u.tokens["reasoning_tokens"] = details["reasoning_tokens"]
		}
	}
	if raw["promptTokenCount"] != nil {
		prompt := pxpipeNum(raw["promptTokenCount"])
		thoughts := pxpipeNum(raw["thoughtsTokenCount"])
		candidates := pxpipeNum(raw["candidatesTokenCount"])
		if candidates == 0 && pxpipeNum(raw["totalTokenCount"]) > 0 {
			candidates = max(0, pxpipeNum(raw["totalTokenCount"])-prompt-thoughts)
		}
		u.tokens["completion_tokens"] = candidates + thoughts
		u.tokens["reasoning_tokens"] = thoughts
	}
	// Claude input_tokens excludes cache reads/writes; storage is cache-inclusive.
	if raw["input_tokens"] != nil && (raw["cache_read_input_tokens"] != nil || raw["cache_creation_input_tokens"] != nil) {
		u.tokens["prompt_tokens"] = pxpipeNum(raw["input_tokens"]) + pxpipeNum(raw["cache_read_input_tokens"]) + pxpipeNum(raw["cache_creation_input_tokens"])
	}
}
func (g *Gateway) finishUsage(r *http.Request, u *requestUsage) {
	if u.live != nil {
		defer u.live.Track(u.model, u.provider, u.connection, false, false)
	}
	u.applyEstimatedUsage()
	if pxpipeNum(u.tokens["prompt_tokens"])+pxpipeNum(u.tokens["completion_tokens"]) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 5*time.Second)
	defer cancel()
	cost := 0.0
	prices, err := (Pricing{Store: database.Pricing{DB: g.DB}}).merged(ctx)
	if err == nil {
		providerPrices, _ := prices[u.provider].(map[string]any)
		if providerPrices == nil {
			providerPrices, _ = prices[providers.AliasMap()[u.provider]].(map[string]any)
		}
		price, _ := providerPrices[u.model].(map[string]any)
		input, output := pxpipeNum(price["input"]), pxpipeNum(price["output"])
		cached, creation := pxpipeNum(u.tokens["cached_tokens"]), pxpipeNum(u.tokens["cache_creation_input_tokens"])
		rate := func(key string, fallback float64) float64 {
			if value, ok := price[key]; ok {
				return pxpipeNum(value)
			}
			return fallback
		}
		cost = (max(0, pxpipeNum(u.tokens["prompt_tokens"])-cached-creation)*input + cached*rate("cached", input) + creation*rate("cache_creation", input) + pxpipeNum(u.tokens["completion_tokens"])*output + pxpipeNum(u.tokens["reasoning_tokens"])*rate("reasoning", output)) / 1e6
	}
	key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if key == "" {
		key = r.Header.Get("x-api-key")
	}
	err = (database.Usage{DB: g.DB}).RecordUsage(ctx, map[string]any{"provider": u.provider, "model": u.model, "connectionId": u.connection, "apiKey": key, "endpoint": r.URL.Path, "tokens": u.tokens, "cost": cost})
	if err != nil {
		slog.Error("failed to persist gateway usage", "error", err)
		return
	}
	if u.live != nil {
		u.live.Notify()
	}
}

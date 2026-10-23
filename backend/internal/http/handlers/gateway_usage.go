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

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type usageContextKey struct{}
type requestUsage struct {
	provider, model, connection string
	tokens                      map[string]any
	live                        *UsageLive
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
	if !b.stream && bytes.HasPrefix(bytes.TrimSpace(b.pending), []byte("data:")) {
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
	if details, ok := raw["prompt_tokens_details"].(map[string]any); ok && details["cached_tokens"] != nil {
		u.tokens["cached_tokens"] = details["cached_tokens"]
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

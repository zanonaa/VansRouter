package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

type PricingStore interface {
	User(context.Context) (map[string]any, error)
	Update(context.Context, map[string]any) (map[string]any, error)
	Reset(context.Context, string, string) error
}
type Pricing struct{ Store PricingStore }

func (p Pricing) merged(ctx context.Context) (map[string]any, error) {
	user, err := p.Store.User(ctx)
	if err != nil {
		return nil, err
	}
	result := providers.DefaultPricing()
	for provider, raw := range user {
		models := jsSpread(raw)
		base, ok := result[provider].(map[string]any)
		if !ok {
			base = map[string]any{}
			result[provider] = base
		}
		for model, value := range models {
			if existing, exists := base[model]; exists {
				merged := jsSpread(existing)
				for k, v := range jsSpread(value) {
					merged[k] = v
				}
				base[model] = merged
			} else {
				base[model] = value
			}
		}
	}
	return result, nil
}
func (p Pricing) Get(w http.ResponseWriter, r *http.Request) {
	value, err := p.merged(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to fetch pricing"})
		return
	}
	nodeReply(w, 200, value)
}
func pricingObject(v any) (map[string]any, bool) {
	switch v.(type) {
	case map[string]any, []any:
		return jsSpread(v), true
	}
	return nil, false
}
func (p Pricing) Patch(w http.ResponseWriter, r *http.Request) {
	fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to update pricing"}) }
	reject := func(message string) { nodeReply(w, 400, map[string]string{"error": message}) }
	decoder := json.NewDecoder(r.Body)
	var raw any
	if decoder.Decode(&raw) != nil {
		fail()
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		fail()
		return
	}
	body, ok := pricingObject(raw)
	if !ok {
		reject("Invalid pricing data format")
		return
	}
	normalized := map[string]any{}
	for provider, rawModels := range body {
		models, ok := pricingObject(rawModels)
		if !ok {
			reject("Invalid pricing for provider: " + provider)
			return
		}
		normalized[provider] = models
		for model, rawPrice := range models {
			price, ok := pricingObject(rawPrice)
			if !ok {
				reject("Invalid pricing for model: " + provider + "/" + model)
				return
			}
			models[model] = price
			for key, value := range price {
				switch key {
				case "input", "output", "cached", "reasoning", "cache_creation":
				default:
					reject(fmt.Sprintf("Invalid pricing field: %s for %s/%s", key, provider, model))
					return
				}
				number, ok := value.(float64)
				if !ok || number < 0 {
					reject(fmt.Sprintf("Invalid pricing value for %s in %s/%s: must be non-negative number", key, provider, model))
					return
				}
			}
		}
	}
	value, err := p.Store.Update(r.Context(), normalized)
	if err != nil {
		fail()
		return
	}
	nodeReply(w, 200, value)
}
func (p Pricing) Delete(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if err := p.Store.Reset(r.Context(), query.Get("provider"), query.Get("model")); err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to reset pricing"})
		return
	}
	value, err := p.merged(r.Context())
	if err != nil {
		nodeReply(w, 500, map[string]string{"error": "Failed to reset pricing"})
		return
	}
	nodeReply(w, 200, value)
}

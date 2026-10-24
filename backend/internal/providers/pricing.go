package providers

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"strings"
)

//go:embed pricing.json
var pricingJSON []byte

//go:embed pricing_fallback.json
var pricingFallbackJSON []byte

// FallbackPricing ports getPricingForModel from open-sse/providers/pricing.js:
// canonical model first, then ordered case-insensitive glob patterns.
func FallbackPricing(model string) map[string]any {
	var catalog struct {
		Models   map[string]map[string]any `json:"models"`
		Patterns []struct {
			Pattern string         `json:"pattern"`
			Pricing map[string]any `json:"pricing"`
		} `json:"patterns"`
	}
	if err := json.Unmarshal(pricingFallbackJSON, &catalog); err != nil {
		panic(err)
	}
	base := model[strings.LastIndex(model, "/")+1:]
	if price := catalog.Models[base]; price != nil {
		return price
	}
	if price := catalog.Models[model]; price != nil {
		return price
	}
	for _, item := range catalog.Patterns {
		pattern := "(?i)^" + strings.ReplaceAll(regexp.QuoteMeta(item.Pattern), `\*`, ".*") + "$"
		match := regexp.MustCompile(pattern)
		if match.MatchString(base) || match.MatchString(model) {
			return item.Pricing
		}
	}
	return nil
}

func DefaultPricing() map[string]any {
	var value map[string]any
	if err := json.Unmarshal(pricingJSON, &value); err != nil {
		panic(err)
	}
	return value
}

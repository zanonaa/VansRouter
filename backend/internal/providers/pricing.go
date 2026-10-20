package providers

import (
	_ "embed"
	"encoding/json"
)

//go:embed pricing.json
var pricingJSON []byte

func DefaultPricing() map[string]any {
	var value map[string]any
	if err := json.Unmarshal(pricingJSON, &value); err != nil {
		panic(err)
	}
	return value
}

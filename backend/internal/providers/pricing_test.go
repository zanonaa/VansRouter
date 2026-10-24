package providers

import "testing"

func TestFallbackPricingJSParity(t *testing.T) {
	for _, tc := range []struct {
		model         string
		input, output float64
	}{
		{"gpt-6.1-sol", 2, 10},
		{"vendor/gpt-6.1-sol", 2, 10},
		{"deepseek-v4-flash", 0.14, 0.28},
	} {
		price := FallbackPricing(tc.model)
		if price["input"] != tc.input || price["output"] != tc.output {
			t.Fatalf("%s: %v", tc.model, price)
		}
	}
	if FallbackPricing("totally-unknown-model") != nil {
		t.Fatal("unknown model should have no price")
	}
}

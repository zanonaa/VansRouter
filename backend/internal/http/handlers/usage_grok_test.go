package handlers

import (
	"encoding/binary"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGrokUsageWrappedValuesAndExhaustedPromo(t *testing.T) {
	u := &UsageContracts{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("x-grok-client-identifier") != "grok-shell" || r.Header.Get("x-email") != "user@example.test" {
			t.Fatal(r.Header)
		}
		body := `{"subscriptionTier":"super_grok"}`
		if strings.Contains(r.URL.Path, "/billing") {
			body = `{"config":{"monthlyLimit":{"val":100},"includedUsed":{"val":25},"onDemandCap":{"val":0},"onDemandUsed":{"val":0},"billingPeriodEnd":"2026-11-01T00:00:00Z"}}`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	result := u.grokUsage(httptest.NewRequest("GET", "/", nil), map[string]any{"accessToken": "token", "providerSpecificData": map[string]any{"email": "user@example.test"}})
	quotas := result["quotas"].(map[string]any)
	if result["plan"] != "Super Grok" || quotas["Monthly included"].(map[string]any)["remainingPercentage"] != float64(75) || quotas["On-demand"] != nil {
		t.Fatal(result)
	}
}

func TestGrokCreditsFrameDecode(t *testing.T) {
	ratio := make([]byte, 4)
	binary.LittleEndian.PutUint32(ratio, math.Float32bits(0.25))
	payload := append([]byte{0x0a, 0x05, 0x0d}, ratio...)
	frame := append([]byte{0, 0, 0, 0, byte(len(payload))}, payload...)
	decoded, err := decodeGrokCredits(frame)
	if err != nil || decoded.percent != 25 {
		t.Fatal(decoded, err)
	}
	if _, err = decodeGrokCredits([]byte{0, 0, 0, 0, 9, 1}); err == nil {
		t.Fatal("accepted truncated frame")
	}
	// Unified-billing payloads carry no usage-ratio field: decoder must return
	// no quota (not an error), matching the JS origin's fail-open null.
	ratioLess := []byte{0, 0, 0, 0, 0x06, 0x0a, 0x04, 0x12, 0x00, 0x1a, 0x00}
	if decoded, decodeErr := decodeGrokCredits(ratioLess); decodeErr != nil || decoded != nil {
		t.Fatal("ratio-less frame must decode to nil without error", decoded, decodeErr)
	}
}

func TestParseGrokFullUsage(t *testing.T) {
	live := map[string]any{"config": map[string]any{"monthlyLimit": map[string]any{"val": float64(0)}, "used": map[string]any{"val": float64(152)}, "billingPeriodEnd": "2026-11-01T00:00:00Z"}}
	quotas, line := parseGrokFullUsage(live)
	if quotas != nil || line == "" || !strings.Contains(line, "152 used this billing period") || !strings.Contains(line, "2026-11-01") {
		t.Fatalf("unified-billing live shape: %v %q", quotas, line)
	}
	withLimit := map[string]any{"config": map[string]any{"monthlyLimit": map[string]any{"val": float64(300)}, "used": map[string]any{"val": float64(152)}, "billingPeriodEnd": "2026-11-01T00:00:00Z"}}
	quotas, line = parseGrokFullUsage(withLimit)
	monthly := quotas["Monthly included"].(map[string]any)
	if line != "" || monthly["remainingPercentage"] != float64(100-152.0/300*100) {
		t.Fatalf("monthly limit shape: %v %q", monthly, line)
	}
	if quotas, line = parseGrokFullUsage(map[string]any{"config": map[string]any{"monthlyLimit": map[string]any{"val": float64(0)}, "used": map[string]any{"val": float64(0)}}}); quotas != nil || line != "" {
		t.Fatalf("empty usage should fall through: %v %q", quotas, line)
	}
}

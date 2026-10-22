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
}

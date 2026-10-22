package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
)

func TestCatalogIPGuard(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.1.2.3", "169.254.169.254", "100.64.0.1", "::1", "::ffff:127.0.0.1", "fc00::1"} {
		if !blockedCatalogIP(netip.MustParseAddr(value)) {
			t.Fatalf("accepted private address %s", value)
		}
	}
	if blockedCatalogIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("blocked public address")
	}
}
func TestProviderSuggestionsFiltersAndRedirect(t *testing.T) {
	h := &ProviderSuggestions{Fetch: func(context.Context, string) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"paid","pricing":{"prompt":"1","completion":"1"},"context_length":300000},{"id":"free","name":"Free","pricing":{"prompt":"0","completion":"0"},"context_length":200000}]}`))}, nil
	}}
	w := httptest.NewRecorder()
	h.Get(w, httptest.NewRequest("GET", "/?url=https://example.invalid/models&type=openrouter-free", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "paid") || !strings.Contains(w.Body.String(), `"id":"free"`) {
		t.Fatal(w.Body.String())
	}
	h.Fetch = func(context.Context, string) (*http.Response, error) {
		return &http.Response{StatusCode: 302, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
	w = httptest.NewRecorder()
	h.Get(w, httptest.NewRequest("GET", "/?url=https://example.invalid/models&type=nvidia-free", nil))
	if w.Code != 502 || !strings.Contains(w.Body.String(), "Model catalog redirected") {
		t.Fatal(w.Body.String())
	}
}

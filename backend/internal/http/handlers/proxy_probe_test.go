package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyProbeUsesProxy(t *testing.T) {
	called := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != "GET" || r.URL.Host != "target.example.test" || r.Header.Get("User-Agent") != "VansRouter" {
			t.Error("proxy request mismatch")
		}
		w.WriteHeader(204)
	}))
	defer proxy.Close()
	result := (ProxyProbe{Targets: []string{"http://target.example.test/check"}}).probe(context.Background(), map[string]any{"proxyUrl": proxy.URL, "type": "http"})
	if !called || !result.OK || result.Status != 204 {
		t.Fatalf("probe: %+v called %v", result, called)
	}
}
func TestRelayProbe(t *testing.T) {
	relay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-relay-target") != "https://httpbin.org" || r.Header.Get("x-relay-path") != "/get" {
			t.Error("relay headers")
		}
		w.WriteHeader(403)
	}))
	defer relay.Close()
	result := (ProxyProbe{}).probe(context.Background(), map[string]any{"proxyUrl": relay.URL, "type": "cloudflare"})
	if result.OK || result.Status != 403 {
		t.Fatalf("relay: %+v", result)
	}
}

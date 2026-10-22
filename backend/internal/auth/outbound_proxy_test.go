package auth

import (
	"os"
	"strings"
	"testing"
)

func proxyEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "NINE_ROUTER_PROXY_MANAGED", "NINE_ROUTER_PROXY_URL", "NINE_ROUTER_NO_PROXY"} {
		t.Setenv(key, "")
	}
}
func TestOutboundProxyClearsOnlyManagedEnvironment(t *testing.T) {
	proxyEnvironment(t)
	os.Setenv("HTTP_PROXY", "http://external:8080")
	applyOutboundProxy(map[string]any{"outboundProxyEnabled": false})
	if os.Getenv("HTTP_PROXY") != "http://external:8080" {
		t.Fatal("cleared unmanaged environment")
	}
	applyOutboundProxy(map[string]any{"outboundProxyEnabled": true, "outboundProxyUrl": "http://managed:8080", "outboundNoProxy": "example.com"})
	if !strings.Contains(os.Getenv("HTTP_PROXY"), "managed") || os.Getenv("NINE_ROUTER_PROXY_MANAGED") != "1" {
		t.Fatal("not managed")
	}
	applyOutboundProxy(map[string]any{"outboundProxyEnabled": true, "outboundProxyUrl": "", "outboundNoProxy": ""})
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "NO_PROXY", "NINE_ROUTER_PROXY_MANAGED", "NINE_ROUTER_PROXY_URL", "NINE_ROUTER_NO_PROXY"} {
		if os.Getenv(key) != "" {
			t.Fatalf("stale %s", key)
		}
	}
}
func TestOutboundProxyRejectsUnsafeValuesAndPreservesExternalSettings(t *testing.T) {
	proxyEnvironment(t)
	os.Setenv("HTTP_PROXY", "http://external:8080")
	for _, proxy := range []string{"file:///tmp/file", "http://host/$secret", "http://host/`command`", "http://host/\nvalue"} {
		applyOutboundProxy(map[string]any{"outboundProxyEnabled": true, "outboundProxyUrl": proxy})
		if os.Getenv("HTTP_PROXY") != "http://external:8080" || os.Getenv("NINE_ROUTER_PROXY_MANAGED") != "" {
			t.Fatalf("unsafe proxy accepted %q", proxy)
		}
	}
}

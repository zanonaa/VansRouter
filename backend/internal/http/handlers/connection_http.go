package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type connectionPoolLookup func(context.Context, string) (map[string]any, error)

// connectionHTTP resolves the single configured pool before legacy settings.
// Rotation strategies remain handled by callers that select a pool beforehand.
func connectionHTTP(ctx context.Context, data map[string]any, lookup connectionPoolLookup, base *http.Client) (*http.Client, string, func(), error) {
	proxyURL, noProxy, relay := "", "", ""
	enabled := data["connectionProxyEnabled"] == true
	if enabled {
		proxyURL = strings.TrimSpace(ttsString(data["connectionProxyUrl"]))
	}
	noProxy = strings.TrimSpace(ttsString(data["connectionNoProxy"]))
	poolID := strings.TrimSpace(ttsString(data["proxyPoolId"]))
	if poolID != "" && poolID != "__none__" && lookup != nil {
		pool, err := lookup(ctx, poolID)
		if err == nil && pool["isActive"] == true && strings.TrimSpace(ttsString(pool["proxyUrl"])) != "" {
			noProxy = ttsString(pool["noProxy"])
			switch pool["type"] {
			case "vercel", "cloudflare", "deno":
				relay = ttsString(pool["proxyUrl"])
				proxyURL = ""
			default:
				proxyURL = ttsString(pool["proxyUrl"])
			}
		}
	}
	if base == nil {
		base = http.DefaultClient
	}
	client := *base
	cleanup := func() {}
	if proxyURL != "" {
		parsed, err := url.Parse(proxyURL)
		if err != nil {
			return nil, "", cleanup, err
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if existing, ok := base.Transport.(*http.Transport); ok {
			transport = existing.Clone()
		}
		transport.Proxy = func(request *http.Request) (*url.URL, error) {
			if connectionNoProxyMatches(request.URL.Hostname(), noProxy) {
				return nil, nil
			}
			return parsed, nil
		}
		client.Transport = transport
		cleanup = transport.CloseIdleConnections
	}
	if usage, ok := ctx.Value(usageContextKey{}).(*requestUsage); ok {
		transport := client.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		client.Transport = usageTransport{base: transport, usage: usage}
	}
	return &client, relay, cleanup, nil
}
func connectionNoProxyMatches(host, list string) bool {
	for _, raw := range strings.Split(list, ",") {
		item := strings.TrimSpace(raw)
		if item == "" {
			continue
		}
		if item == "*" {
			return true
		}
		item = strings.TrimPrefix(item, ".")
		if strings.EqualFold(host, item) || strings.HasSuffix(strings.ToLower(host), "."+strings.ToLower(item)) {
			return true
		}
	}
	return false
}
func rewriteConnectionRelay(request *http.Request, relay string) error {
	if relay == "" {
		return nil
	}
	target, err := url.Parse(relay)
	if err != nil {
		return err
	}
	original := request.URL
	request.Header.Set("x-relay-target", original.Scheme+"://"+original.Host)
	request.Header.Set("x-relay-path", original.RequestURI())
	request.URL = target
	request.Host = ""
	return nil
}

package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

type connectionPoolLookup func(context.Context, string) (map[string]any, error)

// connectionHTTP resolve o pool ativo (com rotação quando configurada) antes
// do proxy legado da conexão. Em strictProxy a requisição nunca cai para o IP
// direto; sem strict, uma falha de transporte no proxy é repetida direto.
func connectionHTTP(ctx context.Context, data map[string]any, lookup connectionPoolLookup, base *http.Client) (*http.Client, string, func(), error) {
	config := resolveConnectionProxy(ctx, data, lookup)
	if base == nil {
		base = http.DefaultClient
	}
	client := *base
	cleanup := func() {}
	transport := client.Transport
	if transport == nil {
		transport = sharedDirectTransport()
	}

	switch {
	case config.relay != "":
		// Relay não usa proxy de ambiente, como o originalFetch do caminho
		// vercel/cloudflare/deno.
		if client.Transport == nil {
			client.Transport = sharedRelayTransport()
		}
	case config.proxyURL != "":
		if err := dialCheck(config.proxyURL); err != nil {
			return nil, "", cleanup, err
		}
		proxyTransport, custom, err := proxyTransport(config.proxyURL, config.noProxy, transport)
		if err != nil {
			return nil, "", cleanup, err
		}
		if custom {
			cleanup = proxyTransport.CloseIdleConnections
		}
		if config.strict {
			client.Transport = proxyTransport
		} else {
			client.Transport = proxyFallbackTransport{proxy: proxyTransport, direct: transport}
		}
	case config.strict && (config.intended || proxyIntended(data)):
		return nil, "", cleanup, errProxyRequired
	default:
		client.Transport = transport
	}

	if usage, ok := ctx.Value(usageContextKey{}).(*requestUsage); ok {
		base := client.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		client.Transport = usageTransport{base: base, usage: usage}
	}
	return &client, config.relay, cleanup, nil
}

// proxyTransport devolve o transporte do proxy. Para o transporte padrão usa
// cache compartilhado (keep-alive reutilizável); para um transporte injetado
// clona e sinaliza que o caller deve fechar as conexões ociosas.
func proxyTransport(proxyURL, noProxy string, base http.RoundTripper) (*http.Transport, bool, error) {
	if base == nil || base == http.RoundTripper(sharedDirectTransport()) {
		transport, _, err := cachedProxyTransport(proxyURL, noProxy)
		return transport, false, err
	}
	custom, ok := base.(*http.Transport)
	if !ok {
		transport, _, err := cachedProxyTransport(proxyURL, noProxy)
		return transport, false, err
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil || parsed.Host == "" {
		return nil, false, err
	}
	transport := custom.Clone()
	transport.Proxy = func(request *http.Request) (*url.URL, error) {
		if connectionNoProxyMatches(request.URL.Hostname(), noProxy) {
			return nil, nil
		}
		return parsed, nil
	}
	return transport, true, nil
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

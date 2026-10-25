package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type ProxyProbe struct {
	// Targets/Transport são injeções exclusivas do teste; produção usa HTTPS
	// público fixo e verificação TLS padrão, nunca URLs fornecidas no request.
	Targets   []string
	Transport func(*url.URL) *http.Transport
}
type probeResult struct {
	OK                bool
	Status            int
	StatusText, Error string
	Elapsed           int64
}

func (p ProxyProbe) probe(ctx context.Context, pool map[string]any) probeResult {
	raw, _ := pool["proxyUrl"].(string)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return probeResult{Status: 400, Error: "proxyUrl is required"}
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" && u.Scheme != "socks5h") {
		return probeResult{Status: 400, Error: "Invalid proxy URL: expected HTTP(S) or SOCKS5 proxy"}
	}
	kind, _ := pool["type"].(string)
	relay := kind == "vercel" || kind == "cloudflare" || kind == "deno"
	transport := &http.Transport{TLSHandshakeTimeout: 8 * time.Second}
	if !relay {
		transport.Proxy = http.ProxyURL(u)
	}
	if isSOCKSProxyURL(raw) {
		transport, _, err = proxyTransport(raw, "", nil)
		if err != nil {
			return probeResult{Status: 400, Error: "Invalid SOCKS5 proxy"}
		}
	}
	if p.Transport != nil {
		transport = p.Transport(u)
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 8 * time.Second}
	targets := p.Targets
	if len(targets) == 0 {
		targets = []string{"https://www.google.com/generate_204", "https://www.gstatic.com/generate_204", "https://detectportal.firefox.com/success.txt"}
	}
	if relay {
		targets = []string{raw}
		client.Timeout = 10 * time.Second
	}
	failures := []string{}
	for _, target := range targets {
		started := time.Now()
		req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
		if err != nil {
			return probeResult{Status: 500, Error: "Invalid test target"}
		}
		if relay {
			req.Header.Set("x-relay-target", "https://httpbin.org")
			req.Header.Set("x-relay-path", "/get")
		} else {
			req.Header.Set("User-Agent", "VansRouter")
		}
		response, err := client.Do(req)
		if err != nil {
			message := "Proxy connection failed"
			if relay {
				message = "Relay connection failed"
			}
			if timeout, ok := err.(interface{ Timeout() bool }); ok && timeout.Timeout() {
				if relay {
					message = "Relay test timed out"
				} else {
					message = "Proxy test timed out"
				}
			}
			if relay {
				return probeResult{Status: 500, Error: message}
			}
			parsed, _ := url.Parse(target)
			failures = append(failures, parsed.Host+": "+message)
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
		response.Body.Close()
		result := probeResult{OK: response.StatusCode >= 200 && response.StatusCode < 300, Status: response.StatusCode, StatusText: http.StatusText(response.StatusCode), Elapsed: time.Since(started).Milliseconds()}
		if relay || result.OK {
			return result
		}
		parsed, _ := url.Parse(target)
		failures = append(failures, fmt.Sprintf("%s: status %d", parsed.Host, response.StatusCode))
	}
	return probeResult{Status: 500, Error: strings.Join(failures, "; ")}
}
func (h ProxyPools) Test(probe ProxyProbe) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fail := func() { nodeReply(w, 500, map[string]string{"error": "Failed to test proxy pool"}) }
		id := chi.URLParam(r, "id")
		pool, err := h.Store.Get(r.Context(), id)
		if err != nil {
			fail()
			return
		}
		if pool == nil {
			nodeReply(w, 404, map[string]string{"error": "Proxy pool not found"})
			return
		}
		result := probe.probe(r.Context(), pool)
		now := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		var lastError any
		if !result.OK {
			lastError = result.Error
			if result.Error == "" {
				lastError = fmt.Sprintf("Proxy test failed with status %d", result.Status)
			}
		}
		if _, err = h.Store.Update(r.Context(), id, map[string]any{"testStatus": map[bool]string{true: "active", false: "error"}[result.OK], "lastTestedAt": now, "lastError": lastError, "isActive": result.OK}); err != nil {
			fail()
			return
		}
		var statusText, errorValue any
		if result.StatusText != "" {
			statusText = result.StatusText
		}
		if result.Error != "" {
			errorValue = result.Error
		}
		nodeReply(w, 200, map[string]any{"ok": result.OK, "status": result.Status, "statusText": statusText, "error": errorValue, "elapsedMs": result.Elapsed, "testedAt": now})
	}
}

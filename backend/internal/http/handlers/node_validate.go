package handlers

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// NodeValidation usa transporte sem proxy de ambiente e fixa o IP após DNS
// para chamadas remotas. Redirecionamentos remotos são recusados: diferença
// deliberada da origem para não contornar a restrição de destinos privados.
type NodeValidation struct{ Local func(*http.Request) bool }

func blockedTarget(ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, cidr := range []string{"0.0.0.0/8", "10.0.0.0/8", "127.0.0.0/8", "169.254.0.0/16", "172.16.0.0/12", "192.168.0.0/16", "100.64.0.0/10", "224.0.0.0/4", "240.0.0.0/4", "::/128", "::1/128", "fe80::/10", "fc00::/7", "ff00::/8"} {
		if netip.MustParsePrefix(cidr).Contains(ip) {
			return true
		}
	}
	return false
}
func validationClient(ctx context.Context, u *url.URL, local bool) (*http.Client, func(), error) {
	transport := &http.Transport{DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext, TLSHandshakeTimeout: 10 * time.Second}
	client := &http.Client{Transport: transport, Timeout: 10 * time.Second}
	if !local {
		host := strings.ToLower(u.Hostname())
		if u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return nil, nil, errors.New("URL not allowed")
		}
		if host == "localhost" || host == "ip6-localhost" || host == "ip6-loopback" {
			return nil, nil, errors.New("URL not allowed")
		}
		for _, suffix := range []string{".internal", ".local", ".localhost"} {
			if strings.HasSuffix(host, suffix) {
				return nil, nil, errors.New("URL not allowed")
			}
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(addresses) == 0 {
			return nil, nil, errors.New("URL not allowed")
		}
		for _, ip := range addresses {
			if blockedTarget(ip) {
				return nil, nil, errors.New("URL not allowed")
			}
		}
		selected := addresses[0].String()
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			target, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			if !strings.EqualFold(target, host) {
				return nil, errors.New("unexpected target")
			}
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, network, net.JoinHostPort(selected, port))
		}
		client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	return client, transport.CloseIdleConnections, nil
}
func validationNetworkError(err error) string {
	if errors.Is(err, syscall.ECONNREFUSED) {
		return "Connection refused - provider node offline or unreachable"
	}
	var dns *net.DNSError
	if errors.As(err, &dns) {
		return "DNS lookup failed - invalid domain or network issue"
	}
	var cert x509.CertificateInvalidError
	if errors.As(err, &cert) && cert.Reason == x509.Expired {
		return "SSL certificate expired"
	}
	var authority x509.UnknownAuthorityError
	if errors.As(err, &authority) {
		return "SSL certificate verification failed"
	}
	var timeout net.Error
	if errors.As(err, &timeout) && timeout.Timeout() {
		return "Request timeout (>10s) - provider node not responding"
	}
	return "Network connection failed - check URL and network connectivity"
}
func modelStatus(status int) string {
	switch status {
	case 401, 403:
		return "API key unauthorized"
	case 404:
		return "Provider has no /models endpoint — enter a Model ID above to validate via chat instead"
	}
	if status >= 500 {
		return "Server error - try again later"
	}
	return fmt.Sprintf("Unexpected response (%d)", status)
}
func chatStatus(status int) string {
	switch status {
	case 401, 403:
		return "API key unauthorized"
	case 400:
		return "Invalid model or bad request"
	case 404:
		return "Chat endpoint not found"
	}
	if status >= 500 {
		return "Server error - try again later"
	}
	return fmt.Sprintf("Chat request failed (%d)", status)
}
func (h NodeValidation) Validate(w http.ResponseWriter, r *http.Request) {
	networkFailure := func(err error) {
		nodeReply(w, 500, map[string]any{"valid": false, "error": validationNetworkError(err)})
	}
	body, err := poolInput(r)
	if err != nil || body == nil {
		networkFailure(errors.New("invalid JSON"))
		return
	}
	if !nodeTruthy(body["baseUrl"]) || !nodeTruthy(body["apiKey"]) {
		nodeReply(w, 400, map[string]string{"error": "Base URL and API key required"})
		return
	}
	base, ok := body["baseUrl"].(string)
	if !ok {
		nodeReply(w, 400, map[string]string{"error": "Invalid URL format"})
		return
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" {
		nodeReply(w, 400, map[string]string{"error": "Invalid URL format"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	client, closeClient, err := validationClient(ctx, parsed, h.Local != nil && h.Local(r))
	if err != nil {
		nodeReply(w, 400, map[string]string{"error": "URL not allowed"})
		return
	}
	defer closeClient()
	key := jsString(body["apiKey"])
	kind, _ := body["type"].(string)
	model := body["modelId"]
	if kind == "anthropic-compatible" {
		base = strings.TrimSuffix(strings.TrimSpace(base), "/")
		base = strings.TrimSuffix(base, "/messages")
		if !strings.HasSuffix(base, "/v1") {
			base += "/v1"
		}
	} else if kind == "custom-embedding" {
		base = strings.TrimSpace(base)
	}
	base = strings.TrimSuffix(base, "/")
	call := func(method, path string, payload any) (int, []byte, error) {
		var data []byte
		if payload != nil {
			data, _ = json.Marshal(payload)
		}
		request, err := http.NewRequestWithContext(r.Context(), method, base+path, bytes.NewReader(data))
		if err != nil {
			return 0, nil, err
		}
		request.Header.Set("Authorization", "Bearer "+key)
		if payload != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		if kind == "anthropic-compatible" {
			request.Header.Set("x-api-key", key)
			request.Header.Set("anthropic-version", "2023-06-01")
		}
		response, err := client.Do(request)
		if err != nil {
			return 0, nil, err
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		return response.StatusCode, raw, err
	}
	if kind == "custom-embedding" {
		modelString, ok := model.(string)
		if !ok && model != nil {
			networkFailure(errors.New("invalid model"))
			return
		}
		if strings.TrimSpace(modelString) == "" {
			nodeReply(w, 200, map[string]any{"valid": false, "error": "Model ID required for embedding validation"})
			return
		}
		status, raw, err := call("POST", "/embeddings", map[string]any{"model": strings.TrimSpace(modelString), "input": "ping"})
		if err != nil {
			networkFailure(err)
			return
		}
		if status >= 200 && status < 300 {
			var payload struct {
				Data []struct {
					Embedding []any `json:"embedding"`
				} `json:"data"`
			}
			var dims any
			if json.Unmarshal(raw, &payload) == nil && len(payload.Data) > 0 && payload.Data[0].Embedding != nil {
				dims = len(payload.Data[0].Embedding)
			}
			nodeReply(w, 200, map[string]any{"valid": true, "method": "embeddings", "dimensions": dims})
			return
		}
		if status == 401 || status == 403 {
			nodeReply(w, 200, map[string]any{"valid": false, "error": "API key unauthorized"})
			return
		}
		message := fmt.Sprintf("Embeddings request failed (%d)", status)
		text := []rune(string(raw))
		if len(text) > 200 {
			text = text[:200]
		}
		if len(text) > 0 {
			message += ": " + string(text)
		}
		nodeReply(w, 200, map[string]any{"valid": false, "method": "embeddings", "error": message})
		return
	}
	status, _, err := call("GET", "/models", nil)
	if err != nil {
		networkFailure(err)
		return
	}
	if status >= 200 && status < 300 {
		nodeReply(w, 200, map[string]bool{"valid": true})
		return
	}
	if status == 401 || status == 403 || !nodeTruthy(model) {
		nodeReply(w, 200, map[string]any{"valid": false, "error": modelStatus(status)})
		return
	}
	status, _, err = call("POST", "/chat/completions", map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "ping"}}, "max_tokens": 1})
	if err != nil {
		networkFailure(err)
		return
	}
	if status >= 200 && status < 300 {
		nodeReply(w, 200, map[string]any{"valid": true, "method": "chat"})
		return
	}
	nodeReply(w, 200, map[string]any{"valid": false, "method": "chat", "error": chatStatus(status)})
}

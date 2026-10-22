package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"time"
)

var suggestionNames = map[string]string{"minimaxai/minimax-m2.7": "MiniMax M2.7", "minimaxai/minimax-m3": "MiniMax M3", "z-ai/glm-5.1": "GLM 5.1", "z-ai/glm-5.2": "GLM 5.2", "deepseek-ai/deepseek-v4-pro": "DeepSeek V4 Pro", "deepseek-ai/deepseek-v4-flash": "DeepSeek V4 Flash", "moonshotai/kimi-k2.6": "Kimi K2.6", "nvidia/nemotron-3-ultra-550b-a55b": "Nemotron 3 Ultra", "nvidia/nv-embedqa-e5-v5": "NV EmbedQA E5 v5"}

func suggestionFilter(kind string, models []map[string]any) []map[string]any {
	out := []map[string]any{}
	for _, model := range models {
		id, idOK := model["id"].(string)
		name := ttsString(model["name"])
		switch kind {
		case "openrouter-free":
			pricing, _ := model["pricing"].(map[string]any)
			if pricing["prompt"] == "0" && pricing["completion"] == "0" && pxpipeNum(model["context_length"]) >= 200000 {
				out = append(out, map[string]any{"id": model["id"], "name": model["name"], "contextLength": model["context_length"]})
			}
		case "opencode-free":
			if strings.HasSuffix(id, "-free") || id == "big-pickle" {
				out = append(out, map[string]any{"id": id, "name": id})
			}
		case "opencode-go":
			if idOK {
				out = append(out, map[string]any{"id": id, "name": id})
			}
		case "mimo-free":
			if strings.HasPrefix(id, "mimo") || strings.Contains(strings.ToLower(name), "mimo") {
				if name == "" {
					name = id
				}
				out = append(out, map[string]any{"id": model["id"], "name": name})
			}
		case "nvidia-free":
			if friendly, ok := suggestionNames[id]; ok {
				out = append(out, map[string]any{"id": id, "name": friendly})
			}
		}
	}
	if kind == "openrouter-free" {
		sort.SliceStable(out, func(i, j int) bool { return pxpipeNum(out[i]["contextLength"]) > pxpipeNum(out[j]["contextLength"]) })
	}
	return out
}

var blockedCatalogRanges = []netip.Prefix{netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("240.0.0.0/4")}

func blockedCatalogIP(ip netip.Addr) bool {
	if ip.Is4In6() {
		ip = ip.Unmap()
	}
	if ip.Is4() {
		for _, prefix := range blockedCatalogRanges {
			if prefix.Contains(ip) {
				return true
			}
		}
		return false
	}
	return ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}

// Fetch fixa o endereço validado no dialer: DNS rebinding e redirects não
// podem converter uma URL pública num acesso interno.
func guardedCatalogFetch(ctx context.Context, rawURL string) (*http.Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("Invalid URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.User != nil {
		return nil, errors.New("Blocked URL: unsupported target")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || host == "ip6-localhost" || host == "ip6-loopback" || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".localhost") {
		return nil, errors.New("Blocked URL: internal host")
	}
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, errors.New("Blocked URL: DNS lookup failed")
	}
	if len(addresses) == 0 {
		return nil, errors.New("Blocked URL: private IP")
	}
	for _, address := range addresses {
		if blockedCatalogIP(address) {
			return nil, errors.New("Blocked URL: private IP")
		}
	}
	port := parsed.Port()
	if port == "" {
		port = "443"
		if parsed.Scheme == "http" {
			port = "80"
		}
	}
	dialer := &net.Dialer{}
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
	}}
	request, err := http.NewRequestWithContext(ctx, "GET", parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	transport.CloseIdleConnections()
	return response, err
}

type ProviderSuggestions struct {
	Fetch func(context.Context, string) (*http.Response, error)
}

func (s *ProviderSuggestions) Get(w http.ResponseWriter, r *http.Request) {
	raw, kind := r.URL.Query().Get("url"), r.URL.Query().Get("type")
	if raw == "" || kind == "" {
		nodeReply(w, 400, map[string]any{"error": "Missing url or type"})
		return
	}
	switch kind {
	case "openrouter-free", "opencode-free", "opencode-go", "mimo-free", "nvidia-free":
	default:
		nodeReply(w, 400, map[string]any{"error": "Unknown filter type"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	fetch := s.Fetch
	if fetch == nil {
		fetch = guardedCatalogFetch
	}
	response, err := fetch(ctx, raw)
	if err != nil {
		message := err.Error()
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			message = "Model catalog timed out"
		}
		nodeReply(w, 502, map[string]any{"error": message})
		return
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 && response.StatusCode < 400 {
		nodeReply(w, 502, map[string]any{"error": "Model catalog redirected"})
		return
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		nodeReply(w, 502, map[string]any{"error": fmt.Sprintf("Model catalog answered %d", response.StatusCode)})
		return
	}
	var payload any
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		nodeReply(w, 502, map[string]any{"error": err.Error()})
		return
	}
	if object, ok := payload.(map[string]any); ok {
		if value := object["data"]; value != nil {
			payload = value
		} else if value := object["models"]; value != nil {
			payload = value
		}
	}
	rows := []map[string]any{}
	if list, ok := payload.([]any); ok {
		for _, item := range list {
			if model, ok := item.(map[string]any); ok {
				rows = append(rows, model)
			}
		}
	}
	nodeReply(w, 200, map[string]any{"data": suggestionFilter(kind, rows)})
}

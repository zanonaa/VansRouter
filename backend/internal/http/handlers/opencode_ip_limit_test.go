package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ip-limit (paridade com chatCore.js + parseError do opencode.js): o free tier
// limita por IP de saída. Em 429/403 FreeUsageLimitError o pool usado é
// marcado unfit no escopo provider::modelo e o próximo pool do pool-set é
// tentado na mesma request (MAX_POOL_RETRIES=2). Sem candidatos, o 429 volta
// para o cliente com Retry-After.

// newIPLimitUpstream sobe um "opencode.ai" de teste em https com o cert do
// httptest; rejeita as failFirst primeiras chamadas com 429 ip-limit.
func newIPLimitUpstream(t *testing.T, failFirst int64) (*httptest.Server, *int64) {
	t.Helper()
	var hits int64
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit := atomic.AddInt64(&hits, 1)
		if hit <= failFirst {
			w.Header().Set("Retry-After", "83000")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"FreeUsageLimitError","message":"Free usage limit reached"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"1\",\"choices\":[{\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	upstream.EnableHTTP2 = false
	upstream.TLS = &tls.Config{NextProtos: []string{"http/1.1"}}
	upstream.StartTLS()
	t.Cleanup(upstream.Close)
	return upstream, &hits
}

// newIPLimitProxy sobe um proxy HTTP que trata CONNECT com túnel TCP ao
// upstream — cada CONNECT é um "IP de saída" diferente aos olhos do executor.
func newIPLimitProxy(t *testing.T, upstream *httptest.Server) *httptest.Server {
	t.Helper()
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			w.WriteHeader(404)
			return
		}
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			w.WriteHeader(500)
			return
		}
		client, _, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer client.Close()
		upstreamConn, err := net.Dial("tcp", upstream.Listener.Addr().String())
		if err != nil {
			client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer upstreamConn.Close()
		if _, err := client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n")); err != nil {
			return
		}
		done := make(chan struct{}, 2)
		go func() { _, _ = io.Copy(upstreamConn, client); done <- struct{}{} }()
		go func() { _, _ = io.Copy(client, upstreamConn); done <- struct{}{} }()
		<-done
	}))
	t.Cleanup(proxy.Close)
	return proxy
}

func trustTestUpstream(t *testing.T, server *httptest.Server) *http.Client {
	t.Helper()
	return &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
}

func TestOpenCodeIPLimitIsDetected(t *testing.T) {
	limitBody := `{"type":"error","error":{"type":"FreeUsageLimitError","message":"Free usage limit reached"}}`
	if !openCodeIsIPLimit(429, limitBody) {
		t.Fatal("429 FreeUsageLimitError deveria ser ip-limit")
	}
	if !openCodeIsIPLimit(403, `{"error":{"message":"rate limited for this ip"}}`) {
		t.Fatal("403 rate deveria ser ip-limit")
	}
	if openCodeIsIPLimit(400, limitBody) {
		t.Fatal("400 nunca é ip-limit")
	}
	if openCodeIsIPLimit(429, `{"error":{"message":"invalid request"}}`) {
		t.Fatal("429 sem corpo de limite não é ip-limit")
	}
}

func TestIPLimitRetryAfterParsing(t *testing.T) {
	now := time.Now().UnixMilli()
	until := ipLimitRetryAfter("3600")
	if until == nil {
		t.Fatal("retry-after numérico deveria produzir until")
	}
	if *until < now || *until > now+3_700_000 {
		t.Fatalf("until implausível: %d", *until)
	}
	fallback := ipLimitRetryAfter("")
	if fallback == nil || *fallback < now+20*3600*1000 {
		t.Fatalf("fallback deveria ser ~24h: %v", fallback)
	}
	if ipLimitRetryAfter("abc") == nil {
		t.Fatal("valor inválido precisa de fallback")
	}
}

func newIPLimitHandler(pools map[string]map[string]any, client *http.Client) *TranslatorSend {
	return &TranslatorSend{
		Connections: sendConnections{[]map[string]any{{
			"provider": "opencode", "isActive": true,
			"providerSpecificData": map[string]any{
				"proxyPoolIds":          []any{"pool-a", "pool-b", "pool-c"},
				"proxyRotationStrategy": "fill-first",
			},
		}}},
		GetPool: func(ctx context.Context, id string) (map[string]any, error) {
			return pools[id], nil
		},
		Client: client,
	}
}

func TestIPLimitMarksPoolUnfitAndRotates(t *testing.T) {
	upstream, hits := newIPLimitUpstream(t, 2)
	var marked []string
	SetProxyFitnessMarker(func(ctx context.Context, poolID, scope string, until float64, reason string) error {
		marked = append(marked, poolID+"/"+reason)
		return nil
	})
	t.Cleanup(func() { SetProxyFitnessMarker(nil) })

	pools := map[string]map[string]any{}
	for _, id := range []string{"pool-a", "pool-b", "pool-c"} {
		proxy := newIPLimitProxy(t, upstream)
		pools[id] = map[string]any{"id": id, "isActive": true, "proxyUrl": proxy.URL, "strictProxy": false, "type": "http"}
	}
	handler := newIPLimitHandler(pools, trustTestUpstream(t, upstream))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(
		`{"model":"beam-free","stream":false,"max_tokens":8,"messages":[{"role":"user","content":"ok"}]}`))
	connections, _ := handler.Connections.List(context.Background(), "opencode", nil)
	handler.sendOpenCodeFree(rec, req, "opencode", "beam-free",
		map[string]any{"messages": []any{map[string]any{"role": "user", "content": "ok"}}, "stream": false, "max_tokens": 8},
		connections[0])

	if len(marked) != 2 {
		t.Fatalf("esperava 2 pools marcados unfit (ip-limit), veio %d: %v (hits=%d)", len(marked), marked, *hits)
	}
	for _, m := range marked {
		if !strings.Contains(m, "/ip-limit") {
			t.Fatalf("motivo inesperado: %s", m)
		}
	}
	if rec.Code != 200 {
		t.Fatalf("esperava 200 após rotação, veio %d: %s", rec.Code, rec.Body.String())
	}
	var completion map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &completion); err != nil {
		t.Fatalf("resposta não é JSON: %s", rec.Body.String())
	}
}

func TestIPLimitReturns429WithRetryAfterWhenAllPoolsFail(t *testing.T) {
	upstream, hits := newIPLimitUpstream(t, 1<<30) // sempre 429
	SetProxyFitnessMarker(func(ctx context.Context, poolID, scope string, until float64, reason string) error {
		return nil
	})
	t.Cleanup(func() { SetProxyFitnessMarker(nil) })

	pools := map[string]map[string]any{}
	for _, id := range []string{"pool-a", "pool-b", "pool-c"} {
		proxy := newIPLimitProxy(t, upstream)
		pools[id] = map[string]any{"id": id, "isActive": true, "proxyUrl": proxy.URL, "strictProxy": false, "type": "http"}
	}
	handler := newIPLimitHandler(pools, trustTestUpstream(t, upstream))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(
		`{"model":"beam-free","stream":false,"max_tokens":8,"messages":[{"role":"user","content":"ok"}]}`))
	connections, _ := handler.Connections.List(context.Background(), "opencode", nil)
	handler.sendOpenCodeFree(rec, req, "opencode", "beam-free",
		map[string]any{"messages": []any{map[string]any{"role": "user", "content": "ok"}}, "stream": false, "max_tokens": 8},
		connections[0])

	if rec.Code != 429 {
		t.Fatalf("esperava 429, veio %d: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Retry-After"); got != "83000" {
		t.Fatalf("Retry-After %q, esperado 83000", got)
	}
	if *hits != 3 {
		t.Fatalf("esperava 3 tentativas upstream (1+2 retries), veio %d", *hits)
	}
}

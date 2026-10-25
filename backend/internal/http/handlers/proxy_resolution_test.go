package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

func clearProxyEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy", "NO_PROXY", "no_proxy"} {
		t.Setenv(key, "")
	}
}

func proxyServer(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("proxied"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("proxy"))
	}))
	t.Cleanup(server.Close)
	return server
}

func targetServer(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("direct"))
	}))
	t.Cleanup(server.Close)
	return server
}

func poolLookup(pools map[string]map[string]any) connectionPoolLookup {
	return func(_ context.Context, id string) (map[string]any, error) {
		return pools[id], nil
	}
}

func doGet(t *testing.T, client *http.Client, target string) (int, string) {
	t.Helper()
	response, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(body)
}

func TestConnectionHTTPUsesProxyPool(t *testing.T) {
	clearProxyEnv(t)
	var proxyHits, targetHits atomic.Int32
	proxy := proxyServer(t, &proxyHits)
	target := targetServer(t, &targetHits)
	pools := map[string]map[string]any{"p1": {"id": "p1", "isActive": true, "proxyUrl": proxy.URL, "type": "http", "strictProxy": false}}
	client, relay, cleanup, err := connectionHTTP(context.Background(), map[string]any{"id": "c1", "proxyPoolId": "p1"}, poolLookup(pools), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if relay != "" {
		t.Fatalf("unexpected relay %q", relay)
	}
	if status, _ := doGet(t, client, target.URL+"/ok"); status != 200 {
		t.Fatalf("status %d", status)
	}
	if proxyHits.Load() != 1 || targetHits.Load() != 0 {
		t.Fatalf("proxy hits=%d target hits=%d", proxyHits.Load(), targetHits.Load())
	}
}

func TestConnectionHTTPNoProxyBypassesPool(t *testing.T) {
	clearProxyEnv(t)
	var proxyHits, targetHits atomic.Int32
	proxy := proxyServer(t, &proxyHits)
	target := targetServer(t, &targetHits)
	host := mustHostname(t, target.URL)
	pools := map[string]map[string]any{"p1": {"id": "p1", "isActive": true, "proxyUrl": proxy.URL, "noProxy": host, "type": "http"}}
	client, _, cleanup, err := connectionHTTP(context.Background(), map[string]any{"id": "c1", "proxyPoolId": "p1"}, poolLookup(pools), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if status, body := doGet(t, client, target.URL+"/ok"); status != 200 || body != "direct" {
		t.Fatalf("status=%d body=%q", status, body)
	}
	if proxyHits.Load() != 0 || targetHits.Load() != 1 {
		t.Fatalf("proxy hits=%d target hits=%d", proxyHits.Load(), targetHits.Load())
	}
}

func TestConnectionHTTPLegacyProxy(t *testing.T) {
	clearProxyEnv(t)
	var proxyHits, targetHits atomic.Int32
	proxy := proxyServer(t, &proxyHits)
	target := targetServer(t, &targetHits)
	client, _, cleanup, err := connectionHTTP(context.Background(), map[string]any{
		"id": "c1", "connectionProxyEnabled": true, "connectionProxyUrl": proxy.URL,
	}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	doGet(t, client, target.URL+"/ok")
	if proxyHits.Load() != 1 || targetHits.Load() != 0 {
		t.Fatalf("proxy hits=%d target hits=%d", proxyHits.Load(), targetHits.Load())
	}
}

func TestConnectionHTTPFallsBackWhenPoolFails(t *testing.T) {
	clearProxyEnv(t)
	var targetHits atomic.Int32
	target := targetServer(t, &targetHits)
	pools := map[string]map[string]any{"p1": {"id": "p1", "isActive": true, "proxyUrl": "http://127.0.0.1:1", "type": "http", "strictProxy": false}}
	client, _, cleanup, err := connectionHTTP(context.Background(), map[string]any{"id": "c1", "proxyPoolId": "p1"}, poolLookup(pools), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if status, body := doGet(t, client, target.URL+"/ok"); status != 200 || body != "direct" {
		t.Fatalf("status=%d body=%q", status, body)
	}
	if targetHits.Load() != 1 {
		t.Fatalf("target hits=%d", targetHits.Load())
	}
}

func TestConnectionHTTPStrictProxyDoesNotFallBack(t *testing.T) {
	clearProxyEnv(t)
	var targetHits atomic.Int32
	target := targetServer(t, &targetHits)
	pools := map[string]map[string]any{"p1": {"id": "p1", "isActive": true, "proxyUrl": "http://127.0.0.1:1", "type": "http", "strictProxy": true}}
	client, _, cleanup, err := connectionHTTP(context.Background(), map[string]any{"id": "c1", "proxyPoolId": "p1"}, poolLookup(pools), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := client.Get(target.URL + "/ok"); err == nil {
		t.Fatal("expected strict proxy failure")
	}
	if targetHits.Load() != 0 {
		t.Fatalf("target hits=%d", targetHits.Load())
	}
}

func TestConnectionHTTPStrictWithInactivePoolFailsBeforeRequest(t *testing.T) {
	clearProxyEnv(t)
	pools := map[string]map[string]any{"p1": {"id": "p1", "isActive": false, "proxyUrl": "http://127.0.0.1:1", "type": "http", "strictProxy": true}}
	_, _, _, err := connectionHTTP(context.Background(), map[string]any{"id": "c1", "proxyPoolId": "p1"}, poolLookup(pools), nil)
	if err != errProxyRequired {
		t.Fatalf("err=%v, want %v", err, errProxyRequired)
	}
}

func TestConnectionHTTPRotationRoundRobin(t *testing.T) {
	clearProxyEnv(t)
	var hits1, hits2 atomic.Int32
	proxy1 := proxyServer(t, &hits1)
	proxy2 := proxyServer(t, &hits2)
	pools := map[string]map[string]any{
		"p1": {"id": "p1", "isActive": true, "proxyUrl": proxy1.URL, "type": "http"},
		"p2": {"id": "p2", "isActive": true, "proxyUrl": proxy2.URL, "type": "http"},
	}
	data := map[string]any{"id": "conn-round-robin", "proxyPoolIds": []any{"p1", "p2"}, "proxyRotationStrategy": "round-robin"}
	for i := 0; i < 4; i++ {
		client, _, cleanup, err := connectionHTTP(context.Background(), data, poolLookup(pools), nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.Get("http://example.test/ok"); err != nil {
			t.Fatal(err)
		}
		cleanup()
	}
	if hits1.Load() != 2 || hits2.Load() != 2 {
		t.Fatalf("hits1=%d hits2=%d, want 2/2", hits1.Load(), hits2.Load())
	}
}

func TestConnectionHTTPRotationSkipsInactivePool(t *testing.T) {
	clearProxyEnv(t)
	var hits1, hits2 atomic.Int32
	proxy1 := proxyServer(t, &hits1)
	proxy2 := proxyServer(t, &hits2)
	pools := map[string]map[string]any{
		"p1": {"id": "p1", "isActive": false, "proxyUrl": proxy1.URL, "type": "http"},
		"p2": {"id": "p2", "isActive": true, "proxyUrl": proxy2.URL, "type": "http"},
	}
	client, _, cleanup, err := connectionHTTP(context.Background(), map[string]any{"id": "conn-skip", "proxyPoolIds": []any{"p1", "p2"}, "proxyRotationStrategy": "fill-first"}, poolLookup(pools), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	client.Get("http://example.test/ok")
	if hits1.Load() != 0 || hits2.Load() != 1 {
		t.Fatalf("hits1=%d hits2=%d", hits1.Load(), hits2.Load())
	}
}

func TestEffectiveProxyUsesGlobalEnvironmentDynamically(t *testing.T) {
	clearProxyEnv(t)
	var hits1, hits2 atomic.Int32
	proxy1 := proxyServer(t, &hits1)
	proxy2 := proxyServer(t, &hits2)
	t.Setenv("HTTP_PROXY", proxy1.URL)
	client, _, cleanup, err := connectionHTTP(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	doGet(t, client, "http://global-proxy.test/ok")
	cleanup()
	t.Setenv("HTTP_PROXY", proxy2.URL)
	client, _, cleanup, err = connectionHTTP(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	doGet(t, client, "http://global-proxy.test/ok")
	cleanup()
	if hits1.Load() != 1 || hits2.Load() != 1 {
		t.Fatalf("hits1=%d hits2=%d, want 1/1", hits1.Load(), hits2.Load())
	}
}

func TestConnectionHTTPHonorsGlobalNoProxy(t *testing.T) {
	clearProxyEnv(t)
	var hits atomic.Int32
	proxy := proxyServer(t, &hits)
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("NO_PROXY", "127.0.0.1")
	client, _, cleanup, err := connectionHTTP(context.Background(), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if _, err := client.Get("http://127.0.0.1:1/ok"); err == nil {
		t.Fatal("expected direct connection failure")
	}
	if hits.Load() != 0 {
		t.Fatalf("proxy hits=%d", hits.Load())
	}
}

func mustHostname(t *testing.T, raw string) string {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed.Hostname()
}

func TestResolvedPoolMatchesActualRoundRobinTransport(t *testing.T) {
	clearProxyEnv(t)
	var a, b atomic.Int32
	pa := proxyServer(t, &a)
	pb := proxyServer(t, &b)
	data := map[string]any{"id": "single-resolution-regression", "proxyPoolIds": []any{"a", "b"}, "proxyRotationStrategy": "round-robin"}
	lookup := func(_ context.Context, id string) (map[string]any, error) {
		raw := pa.URL
		if id == "b" {
			raw = pb.URL
		}
		return map[string]any{"isActive": true, "strictProxy": true, "proxyUrl": raw}, nil
	}
	for _, want := range []string{"a", "b"} {
		client, _, id, cleanup, err := resolveConnectionHTTP(context.Background(), data, lookup, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Get("http://upstream.invalid/ok")
		cleanup()
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if id != want {
			t.Fatalf("reported %s expected %s", id, want)
		}
		if want == "a" && (a.Load() != 1 || b.Load() != 0) {
			t.Fatal("reported pool differs from actual proxy")
		}
	}
	if a.Load() != 1 || b.Load() != 1 {
		t.Fatalf("wrong rotation hits a=%d b=%d", a.Load(), b.Load())
	}
}

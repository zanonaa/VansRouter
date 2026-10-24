package handlers

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// Exercita CONNECT e TLS reais, apenas no namespace loopback do sandbox.
func TestProxyProbeConnectTLS(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/check" {
			t.Error("path mismatch")
		}
		w.WriteHeader(204)
	}))
	defer target.Close()
	targetURL, _ := url.Parse(target.URL)
	var mu sync.Mutex
	connected := false
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "CONNECT" || r.Host != targetURL.Host {
			http.Error(w, "unexpected target", 400)
			return
		}
		upstream, err := net.Dial("tcp", targetURL.Host)
		if err != nil {
			http.Error(w, "target failed", 502)
			return
		}
		defer upstream.Close()
		downstream, buffer, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer downstream.Close()
		mu.Lock()
		connected = true
		mu.Unlock()
		_, _ = buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
		_ = buffer.Flush()
		done := make(chan struct{})
		go func() { _, _ = io.Copy(upstream, buffer); close(done) }()
		_, _ = io.Copy(downstream, upstream)
		downstream.Close()
		upstream.Close()
		<-done
	}))
	defer proxy.Close()
	result := (ProxyProbe{Targets: []string{target.URL + "/check"}, Transport: func(proxyURL *url.URL) *http.Transport {
		// Confia somente na CA fictícia do target, nunca InsecureSkipVerify.
		base := target.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
		return &http.Transport{Proxy: http.ProxyURL(proxyURL), TLSClientConfig: &tls.Config{RootCAs: base.RootCAs, MinVersion: tls.VersionTLS12}}
	}}).probe(context.Background(), map[string]any{"proxyUrl": proxy.URL, "type": "http"})
	mu.Lock()
	defer mu.Unlock()
	if !result.OK || !connected {
		t.Fatalf("CONNECT/TLS failed: %+v connected=%v", result, connected)
	}
}

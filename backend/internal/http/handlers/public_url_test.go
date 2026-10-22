package handlers

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestPublicURLRejectsInternalDNSAndSpecialRanges(t *testing.T) {
	for _, test := range []struct {
		url     string
		ips     []string
		message string
	}{
		{"http://service.example", []string{"10.0.0.1"}, "private IP"},
		{"http://mixed.example", []string{"8.8.8.8", "127.0.0.1"}, "private IP"},
		{"http://[::ffff:127.0.0.1]", nil, "private IP"},
		{"http://100.64.0.1", nil, "private IP"},
		{"http://0.1.2.3", nil, "private IP"},
		{"http://224.0.0.1", nil, "private IP"},
		{"http://240.0.0.1", nil, "private IP"},
		{"http://service.internal", nil, "internal host"},
		{"http://foo.localhost", nil, "internal host"},
		{"http://localhost.", nil, "internal host"},
		{"https://user:pass@example.com", nil, "unsupported target"},
		{"file:///etc/hosts", nil, "Invalid URL"},
	} {
		t.Run(test.url, func(t *testing.T) {
			err := validatePublicURL(context.Background(), test.url, func(context.Context, string) ([]net.IPAddr, error) {
				result := []net.IPAddr{}
				for _, ip := range test.ips {
					result = append(result, net.IPAddr{IP: net.ParseIP(ip)})
				}
				return result, nil
			})
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("got %v wanted %s", err, test.message)
			}
		})
	}
}
func TestPublicURLAllowsOnlyPublicAnswers(t *testing.T) {
	calls := 0
	err := validatePublicURL(context.Background(), "https://public.example/path", func(context.Context, string) ([]net.IPAddr, error) {
		calls++
		return []net.IPAddr{{IP: net.ParseIP("8.8.8.8")}, {IP: net.ParseIP("2606:4700:4700::1111")}}, nil
	})
	if err != nil || calls != 1 {
		t.Fatalf("%v calls %d", err, calls)
	}
	err = validatePublicURL(context.Background(), "https://missing.example", func(context.Context, string) ([]net.IPAddr, error) { return nil, fmt.Errorf("unavailable") })
	if err == nil || !strings.Contains(err.Error(), "DNS lookup failed") {
		t.Fatal(err)
	}
}

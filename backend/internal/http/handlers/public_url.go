package handlers

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

type publicLookup func(context.Context, string) ([]net.IPAddr, error)

var blockedIPv4 = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("169.254.0.0/16"), netip.MustParsePrefix("172.16.0.0/12"), netip.MustParsePrefix("192.168.0.0/16"), netip.MustParsePrefix("100.64.0.0/10"), netip.MustParsePrefix("224.0.0.0/4"), netip.MustParsePrefix("240.0.0.0/4"),
}

func blockedPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if ip.Is4() {
		for _, prefix := range blockedIPv4 {
			if prefix.Contains(ip) {
				return true
			}
		}
		return false
	}
	return ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsMulticast()
}
func publicURL(raw string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return validatePublicURL(ctx, raw, net.DefaultResolver.LookupIPAddr)
}
func validatePublicURL(ctx context.Context, raw string, lookup publicLookup) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return fmt.Errorf("Invalid URL format")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.User != nil {
		return fmt.Errorf("Blocked URL: unsupported target")
	}
	host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
	for _, name := range []string{"localhost", "ip6-localhost", "ip6-loopback"} {
		if host == name {
			return fmt.Errorf("Blocked URL: internal host")
		}
	}
	for _, suffix := range []string{".internal", ".local", ".localhost"} {
		if strings.HasSuffix(host, suffix) {
			return fmt.Errorf("Blocked URL: internal host")
		}
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if blockedPublicIP(ip) {
			return fmt.Errorf("Blocked URL: private IP")
		}
		return nil
	}
	addresses, err := lookup(ctx, host)
	if err != nil {
		return fmt.Errorf("Blocked URL: DNS lookup failed")
	}
	if len(addresses) == 0 {
		return fmt.Errorf("Blocked URL: private IP")
	}
	for _, address := range addresses {
		ip, ok := netip.AddrFromSlice(address.IP)
		if !ok || blockedPublicIP(ip) {
			return fmt.Errorf("Blocked URL: private IP")
		}
	}
	return nil
}

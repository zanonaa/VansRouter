package handlers

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type outboundIdentity struct {
	Provider, Connection string
	Active               bool
}
type outboundAuditTransport struct {
	base   http.RoundTripper
	lookup func(context.Context) (map[[32]byte]outboundIdentity, error)
}

var outboundSequence atomic.Uint64

// InstallOutboundAudit records routing facts only, never raw URLs, headers,
// credentials, prompts, responses or transport errors (which may contain URLs).
// Match credentials in memory to their DB connection without logging hashes.
func InstallOutboundAudit(lookup func(context.Context) (map[[32]byte]outboundIdentity, error)) {
	http.DefaultTransport = outboundAuditTransport{base: http.DefaultTransport, lookup: lookup}
}

// AuditIdentity is intentionally credential-free; keys used by the lookup are
// SHA256 fingerprints and remain in memory only.
type AuditIdentity = outboundIdentity

func CredentialAuditKey(token string) [32]byte { return sha256.Sum256([]byte(token)) }

func (t outboundAuditTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	id := outboundSequence.Add(1)
	provider, connection, model := "", "", ""
	if u, ok := r.Context().Value(usageContextKey{}).(*requestUsage); ok {
		provider, connection, model = u.provider, u.connection, u.model
	}
	matched := outboundIdentity{}
	if t.lookup != nil {
		credentials, err := t.lookup(r.Context())
		if err == nil {
			candidates := []string{strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), r.Header.Get("x-api-key"), r.Header.Get("api-key")}
			for _, token := range candidates {
				if token != "" {
					if identity, ok := credentials[CredentialAuditKey(token)]; ok {
						matched = identity
						break
					}
				}
			}
		}
	}
	// Path allowlist avoids leaking identifiers or secrets embedded in arbitrary
	// provider paths. Unknown paths are deliberately redacted.
	path := "[redacted]"
	for _, safe := range []string{"/api/anthropic/v1/messages", "/api/coding/paas/v4/chat/completions", "/api/paas/v4/chat/completions", "/api/monitor/usage/quota/limit", "/backend-api/codex/responses", "/backend-api/codex/responses/compact", "/backend-api/wham/usage", "/v1/chat/completions"} {
		if r.URL.Path == safe {
			path = safe
			break
		}
	}
	attrs := []any{"audit_id", id, "method", r.Method, "host", r.URL.Hostname(), "path", path, "routed_provider", provider, "routed_model", model, "selected_connection", connection, "credential_provider", matched.Provider, "credential_connection", matched.Connection, "credential_active", matched.Active}
	slog.Info("outbound_start", attrs...)
	start := time.Now()
	response, err := t.base.RoundTrip(r)
	status := 0
	if response != nil {
		status = response.StatusCode
	}
	slog.Info("outbound_headers", "audit_id", id, "status", status, "transport_error", err != nil, "elapsed_ms", time.Since(start).Milliseconds())
	return response, err
}

// Keep proxy transport cloning intact: the shared transport must remain a
// concrete *http.Transport. Audit the completed connection client separately.
func auditConnectionTransport(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = directTransportBase()
	}
	if audit, ok := http.DefaultTransport.(outboundAuditTransport); ok {
		return outboundAuditTransport{base: base, lookup: audit.lookup}
	}
	return base
}
func directTransportBase() http.RoundTripper {
	if audit, ok := http.DefaultTransport.(outboundAuditTransport); ok {
		return audit.base
	}
	return http.DefaultTransport
}

package handlers

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

type auditTestTransport func(*http.Request) (*http.Response, error)

func (f auditTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestOutboundAuditAttributionAndSecretRedaction(t *testing.T) {
	var logs bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	defer slog.SetDefault(old)
	transport := outboundAuditTransport{
		lookup: func(context.Context) (map[[32]byte]outboundIdentity, error) {
			return map[[32]byte]outboundIdentity{CredentialAuditKey("private-key"): {Provider: "glm", Connection: "glm2", Active: false}}, nil
		},
		base: auditTestTransport(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
		}),
	}
	request, _ := http.NewRequest("POST", "https://api.z.ai/api/anthropic/v1/messages?key=private-query", strings.NewReader("private-prompt"))
	request.Header.Set("Authorization", "Bearer private-key")
	request = request.WithContext(context.WithValue(request.Context(), usageContextKey{}, &requestUsage{provider: "codex", connection: "cx1", model: "gpt-test"}))
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	text := logs.String()
	for _, secret := range []string{"private-key", "private-query", "private-prompt"} {
		if strings.Contains(text, secret) {
			t.Fatal("secret leaked")
		}
	}
	for _, expected := range []string{"routed_provider=codex", "credential_provider=glm", "credential_connection=glm2", "credential_active=false", "host=api.z.ai", "status=200"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing attribution %s", expected)
		}
	}
}

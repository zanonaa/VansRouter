package handlers

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestOpenCodeIdentityDisposition(t *testing.T) {
	for _, tt := range []struct{ in, out, want string }{
		{"", "", "absent"}, {"", "generated", "generated_or_defaulted"},
		{"real", "", "dropped"}, {"real", "real", "preserved"}, {"real", "other", "replaced"},
	} {
		if got := openCodeIdentityDisposition(tt.in, tt.out); got != tt.want {
			t.Fatalf("got %s want %s", got, tt.want)
		}
	}
}

func TestOpenCodeIdentityAuditNoValues(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(old)
	t.Setenv("OUTBOUND_AUDIT", "true")
	in := http.Header{}
	in.Set("x-opencode-project", "private-project-secret")
	in.Set("Authorization", "Bearer credential-secret")
	auditOpenCodeIdentity(in, openCodeHeaders(in, "credential-secret"), "pool-test", "deepseek-test", 1)
	got := buf.String()
	if !strings.Contains(got, "project=replaced") || !strings.Contains(got, "pool_id=pool-test") {
		t.Fatalf("missing diagnostic: %s", got)
	}
	for _, secret := range []string{"private-project-secret", "credential-secret"} {
		if strings.Contains(got, secret) {
			t.Fatal("identity or credential leaked")
		}
	}
	buf.Reset()
	t.Setenv("OUTBOUND_AUDIT", "false")
	auditOpenCodeIdentity(in, openCodeHeaders(in, ""), "pool-test", "deepseek-test", 1)
	if buf.Len() != 0 {
		t.Fatal("audit should be opt-in")
	}
}

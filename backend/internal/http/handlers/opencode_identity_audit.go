package handlers

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
)

// Report transformations, never identity values, credentials or request bodies.
// A per-attempt event ties identity handling to the actual resolved pool.
func auditOpenCodeIdentity(in http.Header, out map[string]string, poolID, model string, attempt int) {
	if !strings.EqualFold(os.Getenv("OUTBOUND_AUDIT"), "true") {
		return
	}
	attrs := []any{"pool_id", poolID, "model", model, "attempt", attempt}
	for _, field := range []struct{ header, label string }{
		{"User-Agent", "user_agent"},
		{"x-opencode-client", "client"},
		{"x-opencode-project", "project"},
		{"x-opencode-session", "session"},
		{"x-opencode-request", "request"},
	} {
		attrs = append(attrs, field.label, openCodeIdentityDisposition(in.Get(field.header), out[field.header]))
	}
	slog.Info("opencode_identity", attrs...)
}

func openCodeIdentityDisposition(in, out string) string {
	in = strings.TrimSpace(in)
	switch {
	case in == "" && out == "":
		return "absent"
	case in == "":
		return "generated_or_defaulted"
	case out == "":
		return "dropped"
	case in == out:
		return "preserved"
	default:
		return "replaced"
	}
}

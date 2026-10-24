package auth

import (
	"context"
	"testing"
)

func TestReauthUsesStoredHash(t *testing.T) {
	h := testHTTP(t)
	hash, err := HashPassword("stored-password-123")
	if err != nil {
		t.Fatal(err)
	}
	h.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"password": hash}, nil }
	if h.VerifyDashboardPassword(context.Background(), "123456") {
		t.Fatal("initial password accepted after password set")
	}
	if !h.VerifyDashboardPassword(context.Background(), "stored-password-123") {
		t.Fatal("stored password refused")
	}
}

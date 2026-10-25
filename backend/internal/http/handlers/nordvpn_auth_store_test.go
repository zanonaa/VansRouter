package handlers

import (
	"context"
	"github.com/zanonaa/VansRouter/backend/internal/database"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNordAuthEncryptedAcrossRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = database.Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	store, err := NewNordAuthStore(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	state := nordAuthState{"access-secret", "renew-secret", time.Now().UTC().Truncate(time.Second)}
	if err = store.Save(ctx, state); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err = db.QueryRow(`SELECT encrypted FROM nordvpnAuth`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret") {
		t.Fatal("plaintext stored")
	}
	again, err := NewNordAuthStore(db, dir)
	if err != nil {
		t.Fatal(err)
	}
	got, err := again.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got != state {
		t.Fatalf("restore mismatch: %+v", got.Expires)
	}
	// Idempotent versioned migration preserves credentials.
	if err = database.Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	if _, err = again.Load(ctx); err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 1
	db.Exec(`UPDATE nordvpnAuth SET encrypted=?`, raw)
	if _, err = again.Load(ctx); err == nil {
		t.Fatal("accepted tampered record")
	}
}
func TestNordOAuthRenewSerialized(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/users/tokens/renew" || r.Header.Get("Idempotency-Key") == "" {
			t.Error("invalid renewal contract")
		}
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"token":"new","renew_token":"next","expires_at":"2099-01-01T00:00:00Z"}`))
	}))
	defer server.Close()
	c := &NordClient{BaseURL: server.URL, token: "old", renewToken: "renew", expiresAt: time.Now().Add(-time.Minute)}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.RenewIfDue(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("renewed %d times", calls.Load())
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "new" || c.renewToken != "next" {
		t.Fatal("tokens not rotated")
	}
}

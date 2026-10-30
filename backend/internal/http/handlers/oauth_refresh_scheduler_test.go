package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type refreshSchedStore struct {
	connections []map[string]any
	updated     map[string]map[string]any
}

func (s *refreshSchedStore) List(context.Context, string, *bool) ([]map[string]any, error) {
	return s.connections, nil
}
func (s *refreshSchedStore) Update(_ context.Context, id string, patch map[string]any) (map[string]any, error) {
	if s.updated == nil {
		s.updated = map[string]map[string]any{}
	}
	s.updated[id] = patch
	return nil, nil
}
func (s *refreshSchedStore) DeleteByProvider(context.Context, string) (int64, error) { return 0, nil }

func TestRefreshSchedulerOnlyTouchesDueOAuthConnections(t *testing.T) {
	now := time.Now().UTC()
	store := &refreshSchedStore{connections: []map[string]any{
		// grok expirando em 2min → deve renovar (token endpoint responde 200)
		{"id": "due", "provider": "grok-cli", "accessToken": "x", "refreshToken": "r", "expiresAt": now.Add(2 * time.Minute).Format(time.RFC3339Nano)},
		// claude ainda válido por 1h → não renovar
		{"id": "fresh", "provider": "claude", "accessToken": "x", "refreshToken": "r", "expiresAt": now.Add(time.Hour).Format(time.RFC3339Nano)},
		// sem refresh token → não renovar
		{"id": "norefresh", "provider": "codex", "accessToken": "x", "expiresAt": now.Add(time.Minute).Format(time.RFC3339Nano)},
		// provider sem protocolo OAuth → ignorar mesmo expirado
		{"id": "apikey", "provider": "openai", "accessToken": "k", "expiresAt": now.Add(time.Minute).Format(time.RFC3339Nano)},
	}}
	scheduler := OAuthRefreshScheduler{
		Connections: store,
		Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			if r.URL.Host != "auth.x.ai" {
				t.Fatalf("refresh deve ir ao token endpoint, foi a %s", r.URL.Host)
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"access_token":"fresh","refresh_token":"rotated","expires_in":21600}`))}, nil
		})},
	}
	refreshDueOAuthConnections(context.Background(), scheduler)
	patch, ok := store.updated["due"]
	if !ok {
		t.Fatal("conexão vencida não foi renovada")
	}
	if patch["accessToken"] != "fresh" || patch["refreshToken"] != "rotated" {
		t.Fatalf("patch inesperado: %v", patch)
	}
	if _, ok := store.updated["fresh"]; ok {
		t.Fatal("renovou conexão ainda válida")
	}
	if _, ok := store.updated["norefresh"]; ok {
		t.Fatal("renovou sem refresh token")
	}
	if _, ok := store.updated["apikey"]; ok {
		t.Fatal("renovou provider sem protocolo")
	}
}

func TestRefreshSchedulerFallsBackToExpiresIn(t *testing.T) {
	connection := map[string]any{"provider": "grok-cli", "refreshToken": "r", "expiresIn": float64(21600), "lastRefreshAt": time.Now().UTC().Add(-6 * time.Hour).Format(time.RFC3339Nano)}
	if expiry := parseRefreshExpiry(connection); !expiry.Before(time.Now()) {
		t.Fatalf("esperava expiração derivada de expiresIn passada: %v", expiry)
	}
	if expiry := parseRefreshExpiry(map[string]any{"provider": "grok-cli"}); !expiry.IsZero() {
		t.Fatal("sem dados de expiração deve devolver zero")
	}
}

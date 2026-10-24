package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestQuotaAutoPingOptInAndResetPersistence(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	pings, updates, usage := 0, 0, 0
	connection := map[string]any{"id": "account", "authType": "oauth", "isActive": true}
	opted := false
	q := &QuotaAutoPing{Now: func() time.Time { return now }, Settings: func(context.Context) (map[string]any, error) {
		return map[string]any{"claudeAutoPing": map[string]any{"connections": map[string]any{"account": opted}}}, nil
	}, Connections: func(context.Context, string, *bool) ([]map[string]any, error) {
		return []map[string]any{connection, map[string]any{"id": "other", "authType": "oauth", "isActive": true}}, nil
	}, Usage: func(context.Context, string, map[string]any) (map[string]any, error) {
		usage++
		return map[string]any{"quotas": map[string]any{"session (5h)": map[string]any{"remaining": 90, "resetAt": now.Format(time.RFC3339Nano)}}}, nil
	}, Ping: func(context.Context, string, map[string]any) (bool, error) { pings++; return true, nil }, Update: func(_ context.Context, _ string, patch map[string]any) (map[string]any, error) {
		updates++
		for k, v := range patch {
			connection[k] = v
		}
		return connection, nil
	}}
	if err := q.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if pings != 0 || usage != 0 {
		t.Fatal("called provider without opt-in")
	}
	opted = true
	q.Tick(context.Background())
	q.Tick(context.Background())
	if pings != 1 || updates != 1 || connection["lastPingedResetKey"] != "2026-10-22T12:00:00.000Z" {
		t.Fatalf("pings %d updates %d state %v", pings, updates, connection)
	}
}
func TestCodexAutoPingRequiresSlidingResetAndAvailableWeeklyQuota(t *testing.T) {
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	reset := now.Add(5 * time.Hour)
	remaining := 1.0
	pings := 0
	q := &QuotaAutoPing{Now: func() time.Time { return now }, Settings: func(context.Context) (map[string]any, error) {
		return map[string]any{"codexAutoPing": map[string]any{"connections": map[string]any{"account": true}}}, nil
	}, Connections: func(context.Context, string, *bool) ([]map[string]any, error) {
		return []map[string]any{{"id": "account", "authType": "oauth", "isActive": true}}, nil
	}, Usage: func(context.Context, string, map[string]any) (map[string]any, error) {
		return map[string]any{"quotas": map[string]any{"session": map[string]any{"remaining": 100, "resetAt": reset.Format(time.RFC3339Nano)}, "weekly": map[string]any{"remaining": remaining}}}, nil
	}, Ping: func(context.Context, string, map[string]any) (bool, error) { pings++; return true, nil }, Update: func(context.Context, string, map[string]any) (map[string]any, error) { return nil, nil }}
	q.Tick(context.Background())
	if pings != 0 {
		t.Fatal("pinged without reset history")
	}
	reset = reset.Add(time.Minute)
	remaining = 0
	q.Tick(context.Background())
	if pings != 0 {
		t.Fatal("pinged with exhausted weekly quota")
	}
	reset = reset.Add(time.Minute)
	remaining = 1
	q.Tick(context.Background())
	if pings != 1 {
		t.Fatalf("pings %d", pings)
	}
}
func TestQuotaAutoPingFailureCooldown(t *testing.T) {
	now := time.Now().UTC()
	calls, updates := 0, 0
	q := &QuotaAutoPing{Now: func() time.Time { return now }, Settings: func(context.Context) (map[string]any, error) {
		return map[string]any{"claudeAutoPing": map[string]any{"connections": map[string]any{"account": true}}}, nil
	}, Connections: func(context.Context, string, *bool) ([]map[string]any, error) {
		return []map[string]any{{"id": "account", "authType": "oauth", "isActive": true}}, nil
	}, Usage: func(context.Context, string, map[string]any) (map[string]any, error) {
		return map[string]any{"quotas": map[string]any{"session (5h)": map[string]any{"remaining": 50, "resetAt": now.Format(time.RFC3339Nano)}}}, nil
	}, Ping: func(context.Context, string, map[string]any) (bool, error) {
		calls++
		return false, fmt.Errorf("failed")
	}, Update: func(context.Context, string, map[string]any) (map[string]any, error) { updates++; return nil, nil }}
	q.Tick(context.Background())
	q.Tick(context.Background())
	if calls != 1 || updates != 0 {
		t.Fatal(calls, updates)
	}
	now = now.Add(16 * time.Minute)
	q.Tick(context.Background())
	if calls != 2 {
		t.Fatal(calls)
	}
}

type brokenPingBody struct{}

func (brokenPingBody) Read([]byte) (int, error) { return 0, fmt.Errorf("stream interrupted") }
func (brokenPingBody) Close() error             { return nil }
func TestQuotaPingDrainsSuccessfulBodyBeforeSuccess(t *testing.T) {
	for _, broken := range []bool{false, true} {
		client := &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("ChatGPT-Account-ID") != "workspace" || r.Header.Get("session_id") != "account" {
				t.Fatal(r.Header)
			}
			var body io.ReadCloser = io.NopCloser(strings.NewReader("data: done\n\n"))
			if broken {
				body = brokenPingBody{}
			}
			return &http.Response{StatusCode: 200, Body: body}, nil
		})}
		ok, err := sendQuotaPing(context.Background(), "codex", map[string]any{"id": "account", "accessToken": "token", "providerSpecificData": map[string]any{"workspaceId": "workspace"}}, client, nil)
		if broken && (ok || err == nil) {
			t.Fatal("interrupted stream marked successful")
		}
		if !broken && (!ok || err != nil) {
			t.Fatal(ok, err)
		}
	}
}

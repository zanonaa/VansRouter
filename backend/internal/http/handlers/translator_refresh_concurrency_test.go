package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRefreshDeduplicatesConcurrentAndLaggingRequests(t *testing.T) {
	var calls atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	store := &refreshStore{}
	h := &TranslatorSend{Connections: store, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"new","refresh_token":"rotated"}`))}, nil
	})}}
	connection := map[string]any{"id": "account", "refreshToken": "old"}
	var wg sync.WaitGroup
	results := make(chan map[string]any, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			patch, err := h.refreshConnection(context.Background(), "claude", connection)
			if err != nil {
				t.Error(err)
			}
			results <- patch
		}()
	}
	<-started
	close(release)
	wg.Wait()
	close(results)
	for patch := range results {
		if patch["refreshToken"] != "rotated" {
			t.Fatalf("bad patch %v", patch)
		}
	}
	patch, err := h.refreshConnection(context.Background(), "claude", connection)
	if err != nil || patch["accessToken"] != "new" || calls.Load() != 1 {
		t.Fatalf("lagging request refreshed twice: calls %d err %v", calls.Load(), err)
	}
	patch["accessToken"] = "caller-modified"
	next, _ := h.refreshConnection(context.Background(), "claude", connection)
	if next["accessToken"] != "new" {
		t.Fatal("caller corrupted cached credentials")
	}
}

func TestRefreshWaitingRequestCanCancel(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	h := &TranslatorSend{Connections: &refreshStore{}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-release
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"new"}`))}, nil
	})}}
	connection := map[string]any{"id": "account", "refreshToken": "old"}
	done := make(chan struct{})
	go func() { defer close(done); h.refreshConnection(context.Background(), "claude", connection) }()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := h.refreshConnection(ctx, "claude", connection)
	if err != context.Canceled {
		t.Fatalf("waiter error %v", err)
	}
	close(release)
	<-done
}

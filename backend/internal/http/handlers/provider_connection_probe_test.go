package handlers

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestConnectionProbePersistsState(t *testing.T) {
	store := &gcpStore{item: map[string]any{"id": "c", "provider": "gemini-cli", "accessToken": "token"}}
	status := 200
	probe := &ConnectionProbe{Connections: store, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatal("wrong token")
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("{}"))}, nil
	})}}
	result, err := probe.Test(context.Background(), "c")
	if err != nil || result["valid"] != true || store.patch["testStatus"] != "active" {
		t.Fatalf("result %v err %v", result, err)
	}
	status = 401
	result, err = probe.Test(context.Background(), "c")
	if err != nil || result["error"] != "Token invalid or revoked" || store.patch["testStatus"] != "error" {
		t.Fatalf("result %v err %v", result, err)
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type refreshStore struct {
	patch map[string]any
	fail  bool
}

func (s *refreshStore) List(context.Context, string, *bool) ([]map[string]any, error) {
	return []map[string]any{{"id": "account", "isActive": true, "accessToken": "old", "refreshToken": "refresh-old"}}, nil
}
func (s *refreshStore) Update(_ context.Context, _ string, patch map[string]any) (map[string]any, error) {
	if s.fail {
		return nil, fmt.Errorf("database unavailable")
	}
	s.patch = patch
	return patch, nil
}
func (s *refreshStore) DeleteByProvider(context.Context, string) (int64, error) { return 0, nil }

func TestTranslatorRefreshPersistsAndRetriesOnce(t *testing.T) {
	for _, secondStatus := range []int{200, 401} {
		t.Run(fmt.Sprint(secondStatus), func(t *testing.T) {
			store := &refreshStore{}
			upstream, refreshes := 0, 0
			h := &TranslatorSend{Connections: store, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/v1/oauth/token" || strings.Contains(r.URL.Path, "oauth/token") {
					refreshes++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if body["grant_type"] != "refresh_token" || body["refresh_token"] != "refresh-old" {
						t.Fatalf("refresh payload %v", body)
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"new","refresh_token":"rotated","expires_in":3600}`))}, nil
				}
				upstream++
				expected := "Bearer old"
				status := 401
				if upstream == 2 {
					expected = "Bearer new"
					status = secondStatus
				}
				if r.Header.Get("Authorization") != expected {
					t.Fatalf("token %s expected %s", r.Header.Get("Authorization"), expected)
				}
				raw, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(raw), "full-body") {
					t.Fatal("retry body lost")
				}
				return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
			})}}
			w := httptest.NewRecorder()
			h.Send(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"provider":"claude","model":"model","body":{"messages":[{"role":"user","content":"full-body"}],"max_tokens":123}}`)))
			if w.Code != secondStatus || upstream != 2 || refreshes != 1 || store.patch["refreshToken"] != "rotated" || store.patch["expiresAt"] == nil {
				t.Fatalf("status %d upstream %d refreshes %d patch %v body %s", w.Code, upstream, refreshes, store.patch, w.Body.String())
			}
		})
	}
}
func TestTranslatorDoesNotRetryWhenRefreshPersistenceFails(t *testing.T) {
	store := &refreshStore{fail: true}
	calls := 0
	h := &TranslatorSend{Connections: store, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 401, Body: io.NopCloser(strings.NewReader("unauthorized"))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"access_token":"new"}`))}, nil
	})}}
	w := httptest.NewRecorder()
	h.Send(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"provider":"claude","model":"model","body":{"messages":[]}}`)))
	if w.Code != 500 || calls != 2 || !strings.Contains(w.Body.String(), "persist refreshed") {
		t.Fatalf("%d calls %d %s", w.Code, calls, w.Body.String())
	}
}

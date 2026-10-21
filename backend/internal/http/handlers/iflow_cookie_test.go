package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestIflowCookieRefreshAndMinimizedPersistence(t *testing.T) {
	calls := []string{}
	mode := "success"
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method)
		if r.Header.Get("Cookie") != "Other=dummy; BXAuth=session;" {
			t.Error("normalized cookie", r.Header.Get("Cookie"))
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "GET" {
			if mode == "fetch-denied" {
				w.WriteHeader(403)
				_, _ = w.Write([]byte("denied"))
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"name":"Original"}}`))
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "Original" || r.Header.Get("Origin") != "https://platform.iflow.cn" {
			t.Error(body, r.Header)
		}
		if mode == "refresh-failed" {
			_, _ = w.Write([]byte(`{"success":false,"message":"no refresh"}`))
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"data":{"name":"Refreshed","apiKey":"0123456789-secret-tail","expireTime":"2027-01-01"}}`))
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), Config: func(string) *providers.OAuthConfig { return &providers.OAuthConfig{CookieKeyURL: server.URL} }, CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) {
		saved = data
		data["id"] = "saved"
		return data, nil
	}}
	for _, body := range []string{`{}`, `{"cookie":123}`, `{"cookie":"other=value"}`} {
		w := httptest.NewRecorder()
		h.IflowCookie(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 400 || len(calls) != 0 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	payload := `{"cookie":" Other=dummy; BXAuth=session "}`
	w := httptest.NewRecorder()
	h.IflowCookie(w, httptest.NewRequest("POST", "/", strings.NewReader(payload)))
	if w.Code != 200 || len(calls) != 2 || saved["authType"] != "cookie" || saved["apiKey"] != "0123456789-secret-tail" || strings.Contains(w.Body.String(), "secret-tail") {
		t.Fatal(w.Code, w.Body.String(), saved)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["cookie"] != "BXAuth=session;" || specific["expireTime"] != "2027-01-01" || saved["email"] != "Refreshed" {
		t.Fatal(saved)
	}
	for _, test := range []struct {
		mode   string
		status int
		calls  int
	}{{"fetch-denied", 403, 1}, {"refresh-failed", 400, 2}} {
		mode = test.mode
		saved = nil
		calls = nil
		w = httptest.NewRecorder()
		h.IflowCookie(w, httptest.NewRequest("POST", "/", strings.NewReader(payload)))
		if w.Code != test.status || len(calls) != test.calls || saved != nil {
			t.Fatal(mode, w.Code, w.Body.String(), saved)
		}
	}
}

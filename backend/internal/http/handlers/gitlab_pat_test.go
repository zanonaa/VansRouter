package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGitlabPATVerification(t *testing.T) {
	approved := true
	calls := 0
	var saved map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/v4/user" || r.Header.Get("Private-Token") != "dummy-pat" || r.Header.Get("Authorization") != "" {
			t.Error("PAT request differs", r.URL, r.Header)
		}
		if !approved {
			w.WriteHeader(403)
			_, _ = w.Write([]byte("denied"))
			return
		}
		_, _ = w.Write([]byte(`{"username":"operator","name":"Operator","public_email":"op@example.test"}`))
	}))
	defer server.Close()
	h := OAuthEngine{Client: server.Client(), CreateConnection: func(_ context.Context, data map[string]any) (map[string]any, error) { saved = data; return data, nil }}
	for _, tt := range []struct {
		body   string
		status int
	}{{`invalid`, 400}, {`{}`, 400}, {`{"token":"  "}`, 400}} {
		w := httptest.NewRecorder()
		h.GitlabPAT(w, httptest.NewRequest("POST", "/", strings.NewReader(tt.body)))
		if w.Code != tt.status || calls != 0 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	payload := `{"token":" dummy-pat ","baseUrl":" ` + server.URL + `/ "}`
	w := httptest.NewRecorder()
	h.GitlabPAT(w, httptest.NewRequest("POST", "/", strings.NewReader(payload)))
	if w.Code != 200 || saved["accessToken"] != "dummy-pat" || saved["email"] != "op@example.test" || saved["refreshToken"] != nil || strings.Contains(w.Body.String(), "dummy-pat") {
		t.Fatal(w.Code, w.Body.String(), saved)
	}
	specific := saved["providerSpecificData"].(map[string]any)
	if specific["authKind"] != "personal_access_token" || specific["baseUrl"] != server.URL {
		t.Fatal(specific)
	}
	saved = nil
	approved = false
	w = httptest.NewRecorder()
	h.GitlabPAT(w, httptest.NewRequest("POST", "/", strings.NewReader(payload)))
	if w.Code != 401 || saved != nil || !strings.Contains(w.Body.String(), "denied") {
		t.Fatal(w.Code, w.Body.String(), saved)
	}
}

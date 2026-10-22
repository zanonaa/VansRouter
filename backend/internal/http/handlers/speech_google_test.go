package handlers

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGoogleSpeechRPCAndTokenCache(t *testing.T) {
	tokens, rpcs := 0, 0
	audio := strings.Repeat("a", 100)
	g := &Gateway{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" {
			tokens++
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"FdrFJe":"sid","cfb2h":"build"}`))}, nil
		}
		rpcs++
		if r.URL.Query().Get("f.sid") != "sid" || r.URL.Query().Get("bl") != "build" || r.URL.Query().Get("hl") != "pt" {
			t.Fatal(r.URL)
		}
		r.ParseForm()
		var envelope [][][]any
		if err := json.Unmarshal([]byte(r.Form.Get("f.req")), &envelope); err != nil {
			t.Fatal(err)
		}
		var payload []any
		json.Unmarshal([]byte(envelope[0][0][1].(string)), &payload)
		if payload[0] != "hello . world" {
			t.Fatalf("cleaning %v", payload)
		}
		encoded, _ := json.Marshal([]any{base64.StdEncoding.EncodeToString([]byte(audio))})
		outer, _ := json.Marshal([]any{[]any{"wrb.fr", "jQ1olc", string(encoded)}})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(")]}'\n\n0\n" + string(outer)))}, nil
	})}}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		g.synthesizeGoogle(w, httptest.NewRequest("POST", "/", nil), "pt", map[string]any{"input": "hello@, world"})
		if w.Code != 200 || w.Body.String() != audio {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
	if tokens != 1 || rpcs != 2 {
		t.Fatalf("token calls %d RPC %d", tokens, rpcs)
	}
}
func TestGoogleSpeechRejectsMissingToken(t *testing.T) {
	g := &Gateway{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("no token"))}, nil
	})}}
	w := httptest.NewRecorder()
	g.synthesizeGoogle(w, httptest.NewRequest("POST", "/", nil), "en", map[string]any{"input": "hello"})
	if w.Code != 502 || !strings.Contains(w.Body.String(), "Failed to parse Google token") {
		t.Fatal(w.Body.String())
	}
}

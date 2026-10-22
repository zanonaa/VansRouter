package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEdgeSpeechRetriesRateLimitAndEscapesSSML(t *testing.T) {
	tokens, speech := 0, 0
	audio := strings.Repeat("a", 1024)
	g := &Gateway{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" {
			tokens++
			response := &http.Response{StatusCode: 200, Header: http.Header{"Set-Cookie": {"session=value; Path=/"}}, Body: io.NopCloser(strings.NewReader(`params_AbusePreventionHelper = [` + strings.Repeat("9", tokens) + `,"token",`))}
			return response, nil
		}
		speech++
		r.ParseForm()
		if speech == 1 {
			return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader("limited"))}, nil
		}
		if !strings.Contains(r.Form.Get("ssml"), "hello &amp; &lt;world&gt;") || !strings.Contains(r.Form.Get("ssml"), "xml:gender='Male'") || r.Form.Get("key") != "99" || r.Header.Get("Cookie") != "session=value" {
			t.Fatal(r.Form.Get("ssml"), r.Form, r.Header)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(audio))}, nil
	})}}
	w := httptest.NewRecorder()
	g.synthesizeEdge(w, httptest.NewRequest("POST", "/", nil), "pt-BR-male-Voice", map[string]any{"input": "hello & <world>"})
	if w.Code != 200 || w.Body.String() != audio || tokens != 2 || speech != 2 {
		t.Fatalf("%d tokens %d speech %d %s", w.Code, tokens, speech, w.Body.String())
	}
}

func TestEdgeSpeechRejectsShortAudio(t *testing.T) {
	g := &Gateway{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`params_AbusePreventionHelper = [1,"token",`))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("short"))}, nil
	})}}
	w := httptest.NewRecorder()
	g.synthesizeEdge(w, httptest.NewRequest("POST", "/", nil), "", map[string]any{"input": "hello"})
	if w.Code != 502 || !strings.Contains(w.Body.String(), "empty audio") {
		t.Fatal(w.Body.String())
	}
}

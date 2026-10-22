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

func TestXiaomiSpeechUsesChatAudioPayload(t *testing.T) {
	audio := base64.StdEncoding.EncodeToString([]byte("wav-bytes"))
	g := &Gateway{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		messages := body["messages"].([]any)
		if r.URL.Host != "api.xiaomimimo.com" || body["model"] != "mimo-v2.5-tts" || body["audio"].(map[string]any)["voice"] != "voice" || messages[0].(map[string]any)["content"] != "Speak in Portuguese. calm" || messages[1].(map[string]any)["role"] != "assistant" {
			t.Fatal(body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"audio":{"data":"` + audio + `","format":"wav"}}}]}`))}, nil
	})}}
	w := httptest.NewRecorder()
	g.synthesizeXiaomi(w, httptest.NewRequest("POST", "/", nil), "mimo-v2.5-tts/voice", "token", map[string]any{"input": "hello", "language": "Portuguese", "style": "calm"})
	if w.Code != 200 || w.Body.String() != "wav-bytes" || w.Header().Get("Content-Type") != "audio/wav" {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestXiaomiSpeechRequiresKeyAndAudio(t *testing.T) {
	g := &Gateway{}
	w := httptest.NewRecorder()
	g.synthesizeXiaomi(w, httptest.NewRequest("POST", "/", nil), "model", "", map[string]any{"input": "hello"})
	if w.Code != 400 || !strings.Contains(w.Body.String(), "API key required") {
		t.Fatal(w.Body.String())
	}
	g.Client = &http.Client{Transport: kiroImportTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{}]}`))}, nil
	})}
	w = httptest.NewRecorder()
	g.synthesizeXiaomi(w, httptest.NewRequest("POST", "/", nil), "model", "token", map[string]any{"input": "hello"})
	if w.Code != 502 || !strings.Contains(w.Body.String(), "no audio") {
		t.Fatal(w.Body.String())
	}
}

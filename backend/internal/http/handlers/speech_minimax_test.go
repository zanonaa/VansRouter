package handlers

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestMiniMaxDecodesHexAndRejectsApplicationErrors(t *testing.T) {
	for _, test := range []struct {
		response string
		status   int
		want     string
	}{{`{"base_resp":{"status_code":0},"data":{"audio":"` + hex.EncodeToString([]byte("audio-bytes")) + `"}}`, 200, "audio-bytes"}, {`{"base_resp":{"status_code":1004,"status_msg":"bad key"}}`, 502, "bad key"}, {`{"data":{"audio":"not-hex"}}`, 502, "invalid audio"}} {
		g := &Gateway{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["output_format"] != "hex" || body["stream"] != false || r.Header.Get("Authorization") != "Bearer key" {
				t.Fatal(body)
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(test.response))}, nil
		})}}
		w := httptest.NewRecorder()
		g.synthesizeNative(w, httptest.NewRequest("POST", "/", nil), "minimax", "speech-2.8-hd/voice", "key", providers.MediaConfigs()["minimax"].TTS, map[string]any{"input": "hello"})
		if w.Code != test.status || !strings.Contains(w.Body.String(), test.want) {
			t.Fatalf("%d %s", w.Code, w.Body.String())
		}
	}
}
func TestPlayHTSplitsUserAndKey(t *testing.T) {
	_, headers, payload, _, err := speechPlan("playht", "PlayDialog/voice", "hello", "user:key", providers.MediaConfigs()["playht"].TTS)
	if err != nil || headers["X-USER-ID"] != "user" || headers["Authorization"] != "Bearer key" || payload["voice_engine"] != "PlayDialog" || payload["voice"] != "voice" {
		t.Fatal(headers, payload, err)
	}
}

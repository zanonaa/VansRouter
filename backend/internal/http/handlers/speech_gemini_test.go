package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestGeminiSpeechPCMToWAVAndPrompt(t *testing.T) {
	pcm := []byte{1, 2, 3, 4}
	calls := 0
	g := &Gateway{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		contents := body["contents"].([]any)
		parts := contents[0].(map[string]any)["parts"].([]any)
		if parts[0].(map[string]any)["text"] != "Say in Portuguese: hello" || r.URL.Query().Get("key") != "upstream" || !strings.HasSuffix(r.URL.Path, "gemini-3.1-flash-tts-preview:generateContent") {
			t.Fatal(r.URL, body)
		}
		config := body["generationConfig"].(map[string]any)
		voice := config["speechConfig"].(map[string]any)["voiceConfig"].(map[string]any)["prebuiltVoiceConfig"].(map[string]any)
		if voice["voiceName"] != "Kore" {
			t.Fatal(voice)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"inlineData":{"data":"` + base64.StdEncoding.EncodeToString(pcm) + `"}}]}}]}`))}, nil
	})}}
	w := httptest.NewRecorder()
	g.synthesizeGemini(w, httptest.NewRequest("POST", "/", nil), "gemini-3.1-flash-tts-preview", "upstream", providers.MediaConfigs()["gemini"].TTS, map[string]any{"input": "hello", "language": "Portuguese"})
	wav := w.Body.Bytes()
	if w.Code != 200 || calls != 1 || len(wav) != 48 || string(wav[:4]) != "RIFF" || binary.LittleEndian.Uint32(wav[24:]) != 24000 || binary.LittleEndian.Uint32(wav[40:]) != 4 || !bytes.Equal(wav[44:], pcm) {
		t.Fatalf("bad WAV status %d bytes %v", w.Code, wav)
	}
}
func TestGeminiSpeechMissingAudioReportsReason(t *testing.T) {
	g := &Gateway{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"promptFeedback":{"blockReason":"SAFETY"}}`))}, nil
	})}}
	w := httptest.NewRecorder()
	g.synthesizeGemini(w, httptest.NewRequest("POST", "/", nil), "Kore", "key", providers.MediaConfigs()["gemini"].TTS, map[string]any{"input": "hello"})
	if w.Code != 502 || !strings.Contains(w.Body.String(), "SAFETY") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

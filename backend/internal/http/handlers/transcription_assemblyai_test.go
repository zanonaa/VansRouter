package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestAssemblyAIUploadSubmitPoll(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "audio")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	file.WriteString("audio-bytes")
	file.Seek(0, 0)
	calls := 0
	g := &Gateway{Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("Authorization") != "Bearer upstream" {
			t.Fatal("wrong auth")
		}
		response := ""
		switch calls {
		case 1:
			raw, _ := io.ReadAll(r.Body)
			if string(raw) != "audio-bytes" || r.URL.Path != "/v2/upload" {
				t.Fatal("wrong upload")
			}
			response = `{"upload_url":"https://files.example/audio"}`
		case 2:
			raw, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(raw), `"speech_models":["model"]`) || r.URL.Path != "/v2/transcript" {
				t.Fatal("wrong submit")
			}
			response = `{"id":"job"}`
		case 3:
			response = `{"status":"processing"}`
		case 4:
			response = `{"status":"completed","text":"recognized"}`
		default:
			t.Fatal("unexpected retry")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response))}, nil
	})}}
	w := httptest.NewRecorder()
	g.transcribeAssemblyAI(w, httptest.NewRequest("POST", "/", nil), providers.MediaConfigs()["assemblyai"].STT, file, "model", "upstream", func(context.Context) error { return nil })
	if w.Code != 200 || calls != 4 || strings.TrimSpace(w.Body.String()) != `{"text":"recognized"}` {
		t.Fatalf("%d calls %d %s", w.Code, calls, w.Body.String())
	}
}

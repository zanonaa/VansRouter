package handlers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func TestTranscriptionPreservesOptionsAndTextResponse(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE apiKeys(key TEXT,isActive INTEGER);INSERT INTO apiKeys VALUES('client',1);CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT);INSERT INTO providerConnections VALUES('openai','openai','apikey',NULL,NULL,1,1,'{"apiKey":"provider-key"}','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	fields := map[string]string{"model": "openai/whisper-1", "language": "pt", "prompt": "retain this prompt", "response_format": "text", "temperature": "0.2"}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, _ := form.CreateFormFile("file", "voice.wav")
	part.Write([]byte("audio-content"))
	for k, v := range fields {
		form.WriteField(k, v)
	}
	form.Close()
	calls := 0
	gateway := &Gateway{DB: db, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		defer r.MultipartForm.RemoveAll()
		for k, v := range fields {
			if k == "model" {
				v = "whisper-1"
			}
			if r.FormValue(k) != v {
				t.Fatalf("lost %s: %s", k, r.FormValue(k))
			}
		}
		audio, header, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer audio.Close()
		raw, _ := io.ReadAll(audio)
		if string(raw) != "audio-content" || header.Filename != "voice.wav" {
			t.Fatal("audio modified")
		}
		if _, ok := r.Context().Deadline(); !ok {
			t.Fatal("missing processing deadline")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/plain"}}, Body: io.NopCloser(strings.NewReader("transcribed text"))}, nil
	})}}
	request := httptest.NewRequest("POST", "/api/v1/audio/transcriptions", &body)
	request.Header.Set("Content-Type", form.FormDataContentType())
	request.Header.Set("Authorization", "Bearer client")
	w := httptest.NewRecorder()
	gateway.Transcriptions(w, request)
	if w.Code != 200 || w.Header().Get("Content-Type") != "text/plain" || w.Body.String() != "transcribed text" || calls != 1 {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

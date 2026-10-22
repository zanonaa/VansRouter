package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func TestNativeTranscriptionProtocols(t *testing.T) {
	for _, provider := range []string{"deepgram", "gemini", "huggingface"} {
		t.Run(provider, func(t *testing.T) {
			db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			_, err = db.Exec(`CREATE TABLE settings(id INTEGER PRIMARY KEY,data TEXT);INSERT INTO settings VALUES(1,'{"requireApiKey":false}');CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT)`)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`INSERT INTO providerConnections VALUES('account',?,'apikey',NULL,NULL,1,1,'{"apiKey":"upstream"}','now','now')`, provider)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			g := &Gateway{DB: db, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				raw, _ := io.ReadAll(r.Body)
				response := ""
				switch provider {
				case "deepgram":
					if r.URL.Host != "api.deepgram.com" || r.Header.Get("Authorization") != "Token upstream" || r.URL.Query().Get("language") != "pt" || string(raw) != "audio-content" || r.Header.Get("Content-Type") != "audio/wav" {
						t.Fatalf("wrong Deepgram request %s %s %s", r.URL, r.Header, raw)
					}
					response = `{"results":{"channels":[{"alternatives":[{"transcript":"recognized"}]}]}}`
				case "gemini":
					var payload map[string]any
					if err := json.Unmarshal(raw, &payload); err != nil {
						t.Fatal(err)
					}
					contents := payload["contents"].([]any)
					parts := contents[0].(map[string]any)["parts"].([]any)
					inline := parts[1].(map[string]any)["inline_data"].(map[string]any)
					if inline["data"] != base64.StdEncoding.EncodeToString([]byte("audio-content")) || inline["mime_type"] != "audio/wav" || !strings.Contains(parts[0].(map[string]any)["text"].(string), "custom prompt Language: pt.") || r.URL.Query().Get("key") != "upstream" {
						t.Fatalf("wrong Gemini payload %s", raw)
					}
					response = `{"candidates":[{"content":{"parts":[{"text":"recog"},{"text":"nized"}]}}]}`
				case "huggingface":
					if r.URL.Host != "router.huggingface.co" || !strings.HasSuffix(r.URL.Path, "/organization/model") || string(raw) != "audio-content" || r.Header.Get("Authorization") != "Bearer upstream" {
						t.Fatal("wrong HuggingFace request")
					}
					response = `{"text":"recognized"}`
				}
				if _, ok := r.Context().Deadline(); !ok {
					t.Fatal("missing deadline")
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response))}, nil
			})}}
			var body bytes.Buffer
			form := multipart.NewWriter(&body)
			file, _ := form.CreateFormFile("file", "voice.wav")
			file.Write([]byte("audio-content"))
			model := "model"
			if provider == "huggingface" {
				model = "organization/model"
			}
			form.WriteField("model", provider+"/"+model)
			form.WriteField("language", "pt")
			form.WriteField("prompt", "custom prompt")
			form.Close()
			r := httptest.NewRequest("POST", "/api/v1/audio/transcriptions", &body)
			r.Header.Set("Content-Type", form.FormDataContentType())
			w := httptest.NewRecorder()
			g.Transcriptions(w, r)
			if w.Code != 200 || calls != 1 || strings.TrimSpace(w.Body.String()) != `{"text":"recognized"}` {
				t.Fatalf("%d calls %d %s", w.Code, calls, w.Body.String())
			}
		})
	}
}

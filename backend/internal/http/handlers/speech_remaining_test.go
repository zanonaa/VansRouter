package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func TestPollySignatureStableAndSessionTokenSigned(t *testing.T) {
	request := httptest.NewRequest("POST", "https://polly.us-east-1.amazonaws.com/v1/speech", nil)
	signPolly(request, []byte(`{"Text":"hello"}`), "AKIDEXAMPLE", "secret", "us-east-1", "session", time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))
	auth := request.Header.Get("Authorization")
	if !strings.Contains(auth, "Credential=AKIDEXAMPLE/20261023/us-east-1/polly/aws4_request") || !strings.Contains(auth, "x-amz-security-token") || request.Header.Get("X-Amz-Date") != "20261023T120000Z" {
		t.Fatalf("headers: %v", request.Header)
	}
	first := auth
	signPolly(request, []byte(`{"Text":"different"}`), "AKIDEXAMPLE", "secret", "us-east-1", "session", time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC))
	if first == request.Header.Get("Authorization") {
		t.Fatal("signature did not include payload")
	}
}

func TestRemainingTTSAdapters(t *testing.T) {
	for _, provider := range []string{"aws-polly", "selfhosted-tts"} {
		t.Run(provider, func(t *testing.T) {
			db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			_, err = db.Exec(`CREATE TABLE settings(id INTEGER PRIMARY KEY,data TEXT); INSERT INTO settings VALUES(1,'{"requireApiKey":false}'); CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT);`)
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(`INSERT INTO providerConnections VALUES('test',?,'apikey',NULL,NULL,1,1,?,'now','now')`, provider, `{"apiKey":"secret","providerSpecificData":{"accessKeyId":"AKID","region":"eu-west-1","baseUrl":"http://tts.internal:8880"}}`)
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			g := &Gateway{DB: db, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if provider == "aws-polly" {
					if r.URL.Host != "polly.eu-west-1.amazonaws.com" || body["Engine"] != "neural" || body["VoiceId"] != "Joanna" || body["Text"] != "hello" || !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
						t.Fatalf("request: %s %v %v", r.URL, body, r.Header)
					}
				} else {
					if r.URL.String() != "http://tts.internal:8880/v1/audio/speech" || body["model"] != "kokoro" || body["voice"] != "af_heart" || body["input"] != "hello" || r.Header.Get("Authorization") != "Bearer secret" {
						t.Fatalf("request: %s %v", r.URL, body)
					}
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"audio/mpeg"}}, Body: io.NopCloser(strings.NewReader(strings.Repeat("a", 128)))}, nil
			})}}
			model := provider + "/neural/Joanna"
			if provider == "selfhosted-tts" {
				model = provider + "/kokoro/af_heart"
			}
			w := httptest.NewRecorder()
			g.Speech(w, httptest.NewRequest("POST", "/v1/audio/speech?response_format=json", strings.NewReader(`{"model":"`+model+`","input":"hello"}`)))
			if w.Code != 200 || calls != 1 || !strings.Contains(w.Body.String(), `"format":"mp3"`) {
				t.Fatalf("%d %s calls=%d", w.Code, w.Body.String(), calls)
			}
		})
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestElevenLabsSpeechProtocolAndJSONResponse(t *testing.T) {
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE settings(id INTEGER PRIMARY KEY,data TEXT);INSERT INTO settings VALUES(1,'{"requireApiKey":false}');CREATE TABLE providerConnections(id TEXT PRIMARY KEY,provider TEXT,authType TEXT,name TEXT,email TEXT,priority REAL,isActive INTEGER,data TEXT,createdAt TEXT,updatedAt TEXT);INSERT INTO providerConnections VALUES('account','elevenlabs','apikey',NULL,NULL,1,1,'{"apiKey":"upstream"}','now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	g := &Gateway{DB: db, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if r.URL.Path != "/v1/text-to-speech/voice" || r.Header.Get("xi-api-key") != "upstream" || body["text"] != "hello" || body["model_id"] != "eleven_flash_v2_5" || body["voice_settings"] == nil {
			t.Fatalf("wrong request %s %v", r.URL, body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("a", 1024)))}, nil
	})}}
	r := httptest.NewRequest("POST", "/api/v1/audio/speech?response_format=json", strings.NewReader(`{"model":"el/voice","input":"hello"}`))
	w := httptest.NewRecorder()
	g.Speech(w, r)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	if w.Code != 200 || calls != 1 || result["format"] != "mp3" || result["audio"] == nil {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}
func TestSpeechPlansKeepProviderSpecificAuth(t *testing.T) {
	for _, test := range []struct{ provider, model, header, value string }{{"inworld", "inworld-tts-1.5-mini/Alex", "Authorization", "Basic key"}, {"cartesia", "sonic-2/voice", "X-API-Key", "key"}, {"fish-audio", "s1/voice", "Authorization", "Bearer key"}} {
		cfg := providers.MediaConfigs()[test.provider].TTS
		_, headers, payload, _, err := speechPlan(test.provider, test.model, "text", "key", cfg)
		if err != nil {
			t.Fatal(err)
		}
		if headers[test.header] != test.value || len(payload) == 0 {
			t.Fatal(headers, payload)
		}
	}
}

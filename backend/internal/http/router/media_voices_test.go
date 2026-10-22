package router

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/http/handlers"
)

func TestMediaVoices(t *testing.T) {
	fetchers := map[string]handlers.VoiceFetcher{
		"edge-tts": func(context.Context, string) ([]map[string]any, error) {
			return []map[string]any{{"ShortName": "en-US-Aria", "FriendlyName": "Microsoft Aria Online (Natural) - English (United States)", "Locale": "en-US", "Gender": "F"}}, nil
		},
		"elevenlabs": func(_ context.Context, apiKey string) ([]map[string]any, error) {
			if apiKey != "k" {
				return nil, errBoom
			}
			return []map[string]any{{"voice_id": "x", "name": "X", "labels": map[string]any{"language": "pt-BR", "gender": "neutral"}, "category": "generated"}}, nil
		},
		"local-device": func(context.Context, string) ([]map[string]any, error) {
			return []map[string]any{{"id": "l", "name": "L", "locale": "pt_BR", "lang": "pt", "country": "BR", "gender": "M"}}, nil
		},
	}
	authentication, _ := wiredAuth(t)
	r := NewWithNodes(nil, authentication, nil, &handlers.Providers{Media: &handlers.MediaVoices{Fetchers: fetchers}})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/media-providers/tts/voices?provider=missing", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "does not support voice listing") {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/media-providers/tts/voices", nil))
	var payload struct {
		Voices    []map[string]any          `json:"voices"`
		Languages []map[string]any          `json:"languages"`
		ByLang    map[string]map[string]any `json:"byLang"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	first := payload.Voices[0]
	if first["id"] != "en-US-Aria" || first["name"] != "Aria (English (United States)" || first["lang"] != "en" {
		t.Fatalf("edge shape: %v", first)
	}
	if len(payload.Languages) != 1 || payload.ByLang["en"] == nil {
		t.Fatal(payload)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/media-providers/tts/voices?provider=elevenlabs", nil))
	if w.Code != 502 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/media-providers/tts/voices?provider=elevenlabs&apiKey=k", nil))
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload.Voices[0]["lang"] != "pt" || payload.Voices[0]["gender"] != "neutral" {
		t.Fatalf("eleven shape: %v", payload.Voices[0])
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/media-providers/tts/voices?provider=local-device&lang=pt", nil))
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload.Voices[0]["locale"] != "pt-BR" || payload.Voices[0]["countryName"] != "Brazil" {
		t.Fatalf("local shape: %v", payload.Voices[0])
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/media-providers/tts/voices?provider=local-device&lang=en", nil))
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if len(payload.Voices) != 0 || len(payload.Languages) != 0 {
		t.Fatal("filter ignored")
	}
	authentication.Settings = func(context.Context) (map[string]any, error) { return map[string]any{"requireLogin": true}, nil }
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/media-providers/tts/voices", nil))
	if w.Code != 401 {
		t.Fatal("unguarded")
	}
}

func TestTTSProviderVoiceRoutesRegistered(t *testing.T) {
	authentication, _ := wiredAuth(t)
	paths := []string{"deepgram", "elevenlabs", "inworld", "minimax"}
	r := NewWithNodes(nil, authentication, nil, &handlers.Providers{TTSVoices: &handlers.TTSVoices{}})
	for _, provider := range paths {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/api/media-providers/tts/"+provider+"/voices", nil))
		// Sem conexões o handler responde 400; sem a rota registrada seria 404.
		if w.Code == 404 {
			t.Fatalf("%s rota não registrada: %d %s", provider, w.Code, w.Body.String())
		}
		w = httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("OPTIONS", "/api/media-providers/tts/"+provider+"/voices", nil))
		if w.Code == 404 {
			t.Fatalf("%s options não registrada: %d", provider, w.Code)
		}
	}
}

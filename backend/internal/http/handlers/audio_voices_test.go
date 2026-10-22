package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAudioVoicesValidationAndMapping(t *testing.T) {
	handler := AudioVoices{Media: MediaVoices{Fetchers: map[string]VoiceFetcher{
		"edge-tts": func(context.Context, string) ([]map[string]any, error) {
			return []map[string]any{{"ShortName": "pt-BR-Voice", "FriendlyName": "Microsoft Ana Online (Natural) - Portuguese", "Locale": "pt-BR", "Gender": "Female"}}, nil
		},
	}}}
	w := httptest.NewRecorder()
	handler.Get(w, httptest.NewRequest(http.MethodGet, "/api/v1/audio/voices", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "provider must be one of") || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("validation %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	handler.Get(w, httptest.NewRequest(http.MethodGet, "/api/v1/audio/voices?provider=edge-tts&lang=pt", nil))
	var response struct {
		Data []map[string]any `json:"data"`
	}
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || len(response.Data) != 1 || response.Data[0]["model"] != "edge-tts/pt-BR-Voice" {
		t.Fatalf("mapping %d %s", w.Code, w.Body.String())
	}
}

package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type ttsConnectionsStub struct{ items []map[string]any }

func (s ttsConnectionsStub) List(_ context.Context, _ string, _ *bool) ([]map[string]any, error) {
	return s.items, nil
}

func (s ttsConnectionsStub) Update(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, nil
}

func (s ttsConnectionsStub) DeleteByProvider(context.Context, string) (int64, error) { return 0, nil }

// cannedTransport grava a requisição e responde pela função; evita rede externa.
type ttsCannedTransport struct {
	respond func(req *http.Request) *http.Response
	last    *http.Request
}

func (t *ttsCannedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.last = req
	return t.respond(req), nil
}

func ttsBody(text string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(text)), Header: http.Header{}}
}

func ttsErrorResponse(status int, text string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(text)), Header: http.Header{}}
}

func ttsHarness(t *testing.T, key string) (*TTSVoices, *ttsCannedTransport) {
	t.Helper()
	elevenVoicesCacheMu.Lock()
	elevenVoicesCache = map[string]elevenVoicesCacheEntry{}
	elevenVoicesCacheMu.Unlock()
	transport := &ttsCannedTransport{}
	voices := &TTSVoices{
		Connections: ttsConnectionsStub{items: []map[string]any{{"apiKey": key}}},
		Client:      &http.Client{Transport: transport},
	}
	return voices, transport
}

func TestTTSDeepgramVoices(t *testing.T) {
	voices, transport := ttsHarness(t, "dg-key")
	upstream := `{"tts":[
		{"canonical_name":"aura-2-thalia-en","name":"Thalia","languages":["es","pt"],"metadata":{"tags":["feminine","aura"]}},
		{"canonical_name":"aura-2-luna-en","name":"Luna","languages":["en"],"metadata":{"tags":["masculine"]}}
	]}`
	transport.respond = func(req *http.Request) *http.Response {
		if req.URL.String() != "https://api.deepgram.com/v1/models" || req.Header.Get("Authorization") != "Token dg-key" {
			t.Errorf("upstream request %s %s", req.URL, req.Header.Get("Authorization"))
		}
		return ttsBody(upstream)
	}
	w := httptest.NewRecorder()
	voices.Deepgram(w, httptest.NewRequest("GET", "/api/media-providers/tts/deepgram/voices?lang=es", nil))
	if w.Code != 200 {
		t.Fatalf("lang filter %d %s", w.Code, w.Body.String())
	}
	var filtered struct {
		Sr     string           `json:"-"`
		Voices []map[string]any `json:"voices"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &filtered); err != nil {
		t.Fatal(err)
	}
	if len(filtered.Voices) != 1 || filtered.Voices[0]["id"] != "aura-2-thalia-en" || filtered.Voices[0]["gender"] != "feminine" || filtered.Voices[0]["lang"] != "es" || filtered.Voices[0]["name"] != "Thalia" {
		t.Fatalf("lang filter voices %v", filtered.Voices)
	}
	w = httptest.NewRecorder()
	voices.Deepgram(w, httptest.NewRequest("GET", "/api/media-providers/tts/deepgram/voices", nil))
	var grouped struct {
		Languages []struct {
			Code   string           `json:"code"`
			Name   string           `json:"name"`
			Sr     string           `json:"-"`
			Voices []map[string]any `json:"voices"`
		} `json:"languages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &grouped); err != nil {
		t.Fatal(err)
	}
	if len(grouped.Languages) != 3 {
		t.Fatalf("groups %v", grouped.Languages)
	}
	// Voz duplicada entre grupos: cada grupo tem o próprio id.
	byCode := map[string]int{}
	for _, group := range grouped.Languages {
		byCode[group.Code] = len(group.Voices)
	}
	if byCode["es"] != 1 || byCode["pt"] != 1 || byCode["en"] != 1 {
		t.Fatalf("counts %v", byCode)
	}
	// Sem fallback de nome: canonical_name sem nome usa voiceID como name.
	transport.respond = func(req *http.Request) *http.Response {
		return ttsBody(`{"tts":[{"canonical_name":"en-US-b"}]}`)
	}
	w = httptest.NewRecorder()
	voices.Deepgram(w, httptest.NewRequest("GET", "/api/media-providers/tts/deepgram/voices", nil))
	var fallback struct {
		Languages []struct {
			Code   string           `json:"code"`
			Voices []map[string]any `json:"voices"`
		} `json:"languages"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &fallback)
	if len(fallback.Languages) != 1 || fallback.Languages[0].Code != "b" || fallback.Languages[0].Voices[0]["name"] != "en-US-b" {
		t.Fatalf("fallback groups %+v", fallback.Languages)
	}
}

func TestTTSDeepgramErrors(t *testing.T) {
	voices, transport := ttsHarness(t, "dg-key")
	transport.respond = func(*http.Request) *http.Response {
		return ttsErrorResponse(500, "boom")
	}
	w := httptest.NewRecorder()
	voices.Deepgram(w, httptest.NewRequest("GET", "/api/media-providers/tts/deepgram/voices", nil))
	if w.Code != 502 || !strings.Contains(w.Body.String(), "Deepgram API 500: boom") {
		t.Fatalf("upstream error %d %s", w.Code, w.Body.String())
	}
	// Sem chave ativa: 400 com mensagem exata.
	voicesNoKey := TTSVoices{Connections: ttsConnectionsStub{}, Client: voices.Client}
	w = httptest.NewRecorder()
	voicesNoKey.Deepgram(w, httptest.NewRequest("GET", "/api/media-providers/tts/deepgram/voices", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "No Deepgram connection found") {
		t.Fatalf("no key %d %s", w.Code, w.Body.String())
	}
}

func TestTTSElevenLabsVoices(t *testing.T) {
	voices, transport := ttsHarness(t, "el-key")
	transport.respond = func(req *http.Request) *http.Response {
		if req.URL.String() != "https://api.elevenlabs.io/v1/voices" || req.Header.Get("xi-api-key") != "el-key" {
			t.Errorf("upstream request %s %s", req.URL, req.Header.Get("xi-api-key"))
		}
		return ttsBody(`{"voices":[
			{"voice_id":"v-1","name":"Ana","category":"premade","labels":{"language":"pt-BR","gender":"female"}},
			{"voice_id":"v-2","name":"Owner","category":"generated","is_owner":true,"labels":{"language":"en"},"verified_languages":[{"language":"de"}]}
		]}`)
	}
	w := httptest.NewRecorder()
	voices.ElevenLabs(w, httptest.NewRequest("GET", "/api/media-providers/tts/elevenlabs/voices", nil))
	if w.Code != 200 {
		t.Fatalf("voices %d %s", w.Code, w.Body.String())
	}
	var grouped struct {
		Languages []struct {
			Code   string           `json:"code"`
			Name   string           `json:"name"`
			Voices []map[string]any `json:"voices"`
		} `json:"languages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &grouped); err != nil {
		t.Fatal(err)
	}
	byCode := map[string][]map[string]any{}
	for _, group := range grouped.Languages {
		byCode[group.Code] = group.Voices
	}
	if len(byCode) != 3 {
		t.Fatalf("groups %d: %s", len(byCode), w.Body.String())
	}
	if byCode["pt-BR"][0]["free_users_allowed"] != true {
		t.Fatalf("pt-BR %v", byCode["pt-BR"])
	}
	if byCode["en"][0]["free_users_allowed"] != true {
		t.Fatalf("owner not free %v", byCode["en"][0])
	}
	// A origem reutiliza o free_users_allowed do voice base nos idiomas verificados.
	if byCode["de"][0]["free_users_allowed"] != true || byCode["de"][0]["lang"] != "de" {
		t.Fatalf("verified lang %v", byCode["de"][0])
	}
	// Cache de 24h: segunda chamada não aciona o transporte (responde stale).
	transport.respond = nil
	w = httptest.NewRecorder()
	voices.ElevenLabs(w, httptest.NewRequest("GET", "/api/media-providers/tts/elevenlabs/voices", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "v-1") {
		t.Fatalf("cache reuso %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	voices.ElevenLabs(w, httptest.NewRequest("GET", "/api/media-providers/tts/elevenlabs/voices?lang=de", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"lang":"de"`) || strings.Contains(w.Body.String(), `"en"`) {
		t.Fatalf("lang filter de %d %s", w.Code, w.Body.String())
	}
}

func TestTTSElevenLabsFetchFailure(t *testing.T) {
	voices, transport := ttsHarness(t, "bad-key")
	transport.respond = func(*http.Request) *http.Response {
		return ttsErrorResponse(401, "denied")
	}
	w := httptest.NewRecorder()
	voices.ElevenLabs(w, httptest.NewRequest("GET", "/api/media-providers/tts/elevenlabs/voices", nil))
	if w.Code != 502 || !strings.Contains(w.Body.String(), "ElevenLabs voices fetch failed: 401") {
		t.Fatalf("fetch fail %d %s", w.Code, w.Body.String())
	}
	// Sem chave: 400.
	noKey := TTSVoices{Connections: ttsConnectionsStub{}, Client: voices.Client}
	w = httptest.NewRecorder()
	noKey.ElevenLabs(w, httptest.NewRequest("GET", "/api/media-providers/tts/elevenlabs/voices", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "No ElevenLabs connection found") {
		t.Fatalf("no key %d %s", w.Code, w.Body.String())
	}
}

func TestTTSInworldVoices(t *testing.T) {
	voices, transport := ttsHarness(t, "iw-key")
	transport.respond = func(req *http.Request) *http.Response {
		if req.URL.String() != "https://api.inworld.ai/tts/v1/voices" || req.Header.Get("Authorization") != "Basic iw-key" {
			t.Errorf("upstream request %s %s", req.URL, req.Header.Get("Authorization"))
		}
		return ttsBody(`{"voices":[
			{"voiceId":"iw-1","displayName":"Inworld One","languages":["pt","es"],"gender":"MALE"},
			{"voiceId":"iw-2","languages":["en"],"gender":""}
		]}`)
	}
	w := httptest.NewRecorder()
	voices.Inworld(w, httptest.NewRequest("GET", "/api/media-providers/tts/inworld/voices", nil))
	if w.Code != 200 {
		t.Fatalf("voices %d %s", w.Code, w.Body.String())
	}
	var grouped struct {
		Languages []struct {
			Code   string           `json:"code"`
			Voices []map[string]any `json:"voices"`
		} `json:"languages"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &grouped)
	byCode := map[string]map[string]any{}
	for _, group := range grouped.Languages {
		byCode[group.Code] = group.Voices[0]
	}
	if byCode["pt"]["name"] != "Inworld One" || byCode["es"]["gender"] != "MALE" {
		t.Fatalf("groups %v", byCode)
	}
	if byCode["en"]["name"] != "iw-2" || byCode["en"]["gender"] != "" {
		t.Fatalf("fallback name %v", byCode["en"])
	}
	transport.respond = func(*http.Request) *http.Response {
		return ttsErrorResponse(503, "")
	}
	w = httptest.NewRecorder()
	voices.Inworld(w, httptest.NewRequest("GET", "/api/media-providers/tts/inworld/voices", nil))
	if w.Code != 502 || !strings.Contains(w.Body.String(), "Inworld API 503: Failed") {
		t.Fatalf("upstream error %d %s", w.Code, w.Body.String())
	}
	noKey := TTSVoices{Connections: ttsConnectionsStub{}, Client: voices.Client}
	w = httptest.NewRecorder()
	noKey.Inworld(w, httptest.NewRequest("GET", "/api/media-providers/tts/inworld/voices", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "No Inworld connection found") {
		t.Fatalf("no key %d %s", w.Code, w.Body.String())
	}
}

func TestTTSMiniMaxVoices(t *testing.T) {
	voices, transport := ttsHarness(t, "mm-key")
	upstream := `{
		"system_voice":[{"voice_id":"Chinese_Mandarin_1","voice_name":"CM One"},{"voice_id":"Mano","voice_name":"Mano"}],
		"voice_cloning":[{"voiceId":"clone-9","voiceName":"My Clone"}],
		"unknown_group":[{"voice_id":"ignored"}]
	}`
	transport.respond = func(req *http.Request) *http.Response {
		if req.URL.String() != "https://api.minimax.io/v1/get_voice" || req.Method != "POST" {
			t.Errorf("upstream request %s %s", req.Method, req.URL)
		}
		var body map[string]string
		_ = json.NewDecoder(req.Body).Decode(&body)
		if body["voice_type"] != "all" {
			t.Errorf("voice_type %v", body)
		}
		if req.Header.Get("Authorization") != "Bearer mm-key" || req.Header.Get("Content-Type") != "application/json" {
			t.Errorf("headers %v", req.Header)
		}
		return ttsBody(upstream)
	}
	w := httptest.NewRecorder()
	voices.MiniMax(w, httptest.NewRequest("GET", "/api/media-providers/tts/minimax/voices", nil))
	if w.Code != 200 {
		t.Fatalf("voices %d %s", w.Code, w.Body.String())
	}
	var grouped struct {
		Languages []struct {
			Code   string           `json:"code"`
			Name   string           `json:"name"`
			Voices []map[string]any `json:"voices"`
		} `json:"languages"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &grouped); err != nil {
		t.Fatal(err)
	}
	if len(grouped.Languages) < 2 {
		t.Fatalf("groups %s", w.Body.String())
	}
	// Custom deve ser o último language no array ordenado.
	if last := grouped.Languages[len(grouped.Languages)-1]; last.Code != "Custom" {
		t.Fatalf("custom last, got %s", last.Code)
	}
	byCode := map[string][]map[string]any{}
	for _, group := range grouped.Languages {
		byCode[group.Code] = group.Voices
	}
	if byCode["Chinese"][0]["name"] != "CM One" || byCode["Chinese"][0]["id"] != "Chinese_Mandarin_1" {
		t.Fatalf("system group %v", byCode["Chinese"])
	}
	customByID := map[string]map[string]any{}
	for _, voice := range byCode["Custom"] {
		customByID[voice["id"].(string)] = voice
	}
	if customByID["clone-9"]["name"] != "My Clone · Cloned" || customByID["clone-9"]["category"] != "voice_cloning" {
		t.Fatalf("custom group %v", byCode["Custom"])
	}
	if customByID["Mano"]["category"] != "system_voice" || customByID["Mano"]["name"] != "Mano" {
		t.Fatalf("system no underscore %v", customByID["Mano"])
	}
}

func TestTTSMiniMaxVariants(t *testing.T) {
	voices, transport := ttsHarness(t, "mm-key")
	// base_resp com erro → 502.
	transport.respond = func(*http.Request) *http.Response {
		return ttsBody(`{"base_resp":{"status_code":1004,"status_msg":"invalid key"}}`)
	}
	w := httptest.NewRecorder()
	voices.MiniMax(w, httptest.NewRequest("GET", "/api/media-providers/tts/minimax/voices", nil))
	if w.Code != 502 {
		t.Fatalf("base resp error %d %s", w.Code, w.Body.String())
	}
	var baseRespErr map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &baseRespErr); err != nil || baseRespErr["error"] != "invalid key" {
		t.Fatalf("base resp error %d %v err=%v", w.Code, baseRespErr, err)
	}
	// minimax-cn usa o endpoint .com.br/.com chinês.
	transport.respond = func(req *http.Request) *http.Response {
		if req.URL.String() != "https://api.minimaxi.com/v1/get_voice" {
			t.Errorf("endpoint %s", req.URL)
		}
		return ttsBody(`{"base_resp":{"status_code":0},"system_voice":[{"voice_id":"Mano_Zh","voice_name":"Mano"}],"voice_cloning":[{"voiceId":"c1","voiceName":"X"}]}`)
	}
	w = httptest.NewRecorder()
	voices.MiniMax(w, httptest.NewRequest("GET", "/api/media-providers/tts/minimax/voices?provider=minimax-cn&lang=Mano", nil))
	wCode, wBody := w.Code, w.Body.String()
	if wCode != 200 {
		t.Fatalf("cn lang filter %d %s", wCode, wBody)
	}
	var cnFiltered struct {
		Sr     string           `json:"-"`
		Voices []map[string]any `json:"voices"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &cnFiltered); err != nil {
		t.Fatal(err)
	}
	if len(cnFiltered.Voices) != 1 || cnFiltered.Voices[0]["id"] != "Mano_Zh" || cnFiltered.Voices[0]["lang"] != "Mano" || cnFiltered.Voices[0]["category"] != "system_voice" || cnFiltered.Voices[0]["name"] != "Mano" {
		t.Fatalf("cn filtered %v", cnFiltered.Voices)
	}
	// lang inexistente volta vazio (mesmo endpoint cn válido).
	w = httptest.NewRecorder()
	voices.MiniMax(w, httptest.NewRequest("GET", "/api/media-providers/tts/minimax/voices?provider=minimax-cn&lang=Never", nil))
	if w.Code != 200 {
		t.Fatalf("unknown lang %d %s", w.Code, w.Body.String())
	}
	var empty struct {
		Sr     string           `json:"-"`
		Voices []map[string]any `json:"voices"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &empty); err != nil || len(empty.Voices) != 0 {
		t.Fatalf("unknown lang %d %s", w.Code, w.Body.String())
	}
	// Cabeçalho HTTP não-2xx com texto cru.
	transport.respond = func(*http.Request) *http.Response {
		return ttsErrorResponse(500, "raw-failure")
	}
	w = httptest.NewRecorder()
	voices.MiniMax(w, httptest.NewRequest("GET", "/api/media-providers/tts/minimax/voices", nil))
	if w.Code != 502 || !strings.Contains(w.Body.String(), "MiniMax API 500: raw-failure") {
		t.Fatalf("http error %d %s", w.Code, w.Body.String())
	}
	// Sem chave: mensagem usa o provider minúsculo.
	noKey := TTSVoices{Connections: ttsConnectionsStub{}, Client: voices.Client}
	w = httptest.NewRecorder()
	noKey.MiniMax(w, httptest.NewRequest("GET", "/api/media-providers/tts/minimax/voices?provider=minimax-cn", nil))
	if w.Code != 400 || !strings.Contains(w.Body.String(), "No minimax-cn connection found") {
		t.Fatalf("no key %d %s", w.Code, w.Body.String())
	}
}

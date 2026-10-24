package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

func TestChatComboFallbackAndStickyRotation(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}'`)
	_, err := g.DB.Exec(`CREATE TABLE IF NOT EXISTS combos(id TEXT PRIMARY KEY,name TEXT,kind TEXT,models TEXT,context_length REAL,createdAt TEXT,updatedAt TEXT);INSERT INTO combos VALUES('combo','bundle','llm','["openai/first","openai/second"]',NULL,'now','now');`)
	if err != nil {
		t.Fatal(err)
	}
	requests := []string{}
	failFirst := true
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		text := string(raw)
		model := "second"
		if strings.Contains(text, `"model":"first"`) {
			model = "first"
		}
		requests = append(requests, model)
		status := 200
		response := "data: success\n\n"
		if model == "first" && failFirst {
			status = 429
			response = "failed-candidate-must-not-leak"
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(response))}, nil
	})}}
	invoke := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"bundle","messages":[{"role":"user","content":"hello"}]} `)))
		return w
	}
	w := invoke()
	if w.Code != 200 || len(requests) != 2 || strings.Contains(w.Body.String(), "must-not-leak") {
		t.Fatalf("fallback %d %v %s", w.Code, requests, w.Body.String())
	}
	failFirst = false
	requests = nil
	providers.ResetComboRotation("bundle")
	defer providers.ResetComboRotation("bundle")
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false,"comboStrategy":"round-robin","comboStickyRoundRobinLimit":2}'`)
	for i := 0; i < 4; i++ {
		if w := invoke(); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	if strings.Join(requests, ",") != "first,first,second,second" {
		t.Fatal(requests)
	}
}

// accountFallback.js só cai para o próximo modelo em 401/402/403/404/429 e
// 5xx. Um 4xx sem regra (410, 408, 409, 413, ...) é erro do pedido e deve ser
// devolvido sem consumir outro candidato.
func TestComboDoesNotFallbackOnUnmatched4xx(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}'`)
	if _, err := g.DB.Exec(`INSERT INTO combos VALUES('c','bundle','llm','["openai/primary","openai/second"]',NULL,'now','now');`); err != nil {
		t.Fatal(err)
	}
	requests := []string{}
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		model := "second"
		if strings.Contains(string(raw), `"model":"primary"`) {
			model = "primary"
		}
		requests = append(requests, model)
		if model == "primary" {
			return &http.Response{StatusCode: 410, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":"model gone"}`))}, nil
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
	})}}
	w := httptest.NewRecorder()
	g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"bundle","messages":[{"role":"user","content":"hi"}]}`)))
	if w.Code != 410 {
		t.Fatalf("status %d, esperado 410: %s", w.Code, w.Body.String())
	}
	if strings.Join(requests, ",") != "primary" {
		t.Fatalf("não deveria tentar o segundo candidato: %v", requests)
	}
}

// Fusion com um único modelo era reentrante: o body continuava com o nome do
// combo e OpenAIChat chamava o combo de novo. Deve fixar o modelo do painel.
func TestFusionSingleModelUsesConfiguredModel(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false,"comboStrategy":"fusion"}'`)
	if _, err := g.DB.Exec(`INSERT INTO combos VALUES('c','solo','llm','["openai/solo"]',NULL,'now','now');`); err != nil {
		t.Fatal(err)
	}
	requests := []string{}
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		model := ""
		if strings.Contains(string(raw), `"model":"solo"`) {
			model = "solo"
		}
		requests = append(requests, model)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
	})}}
	w := httptest.NewRecorder()
	g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"solo","messages":[{"role":"user","content":"hi"}]}`)))
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(requests) != 1 || requests[0] != "solo" {
		t.Fatalf("model do painel não foi fixado: %v", requests)
	}
}

// Quando todos os painéis já terminaram (mesmo com falha), collectPanel na
// origem encerra imediatamente; não há straggler para esperar.
func TestFusionAllPanelFailuresDoesNotWaitGrace(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false,"comboStrategy":"fallback","comboStrategies":{"bundle":{"fallbackStrategy":"fusion","fusionTuning":{"stragglerGraceMs":5000}}}}'`)
	if _, err := g.DB.Exec(`INSERT INTO combos VALUES('c','bundle','llm','["openai/first","openai/second"]',NULL,'now','now');`); err != nil {
		t.Fatal(err)
	}
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 500, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":"boom"}`))}, nil
	})}}
	start := time.Now()
	w := httptest.NewRecorder()
	g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"bundle","messages":[{"role":"user","content":"hi"}]}`)))
	if w.Code != 503 {
		t.Fatalf("status %d, esperado 503: %s", w.Code, w.Body.String())
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("esperou o grace sem stragglers: %s", elapsed)
	}
}

func TestComboPermissionBlocksBeforeCandidate(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`CREATE TABLE IF NOT EXISTS combos(id TEXT PRIMARY KEY,name TEXT,kind TEXT,models TEXT,context_length REAL,createdAt TEXT,updatedAt TEXT);INSERT INTO combos VALUES('c','bundle','llm','["openai/model"]',NULL,'now','now');`)
	key, err := database.CreateApiKey(context.Background(), g.DB, "combo", "machine")
	if err != nil {
		t.Fatal(err)
	}
	database.UpdateApiKey(context.Background(), g.DB, ttsString(key["id"]), map[string]any{"allowedCombos": []any{}})
	g.Send = &TranslatorSend{}
	r := httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"bundle","messages":[]}`))
	r.Header.Set("Authorization", "Bearer "+ttsString(key["key"]))
	w := httptest.NewRecorder()
	g.OpenAIChat(w, r)
	if w.Code != 403 || !strings.Contains(w.Body.String(), `Combo`) {
		t.Fatalf("%d %s", w.Code, w.Body.String())
	}
}

func TestComboListedIdIsRoutable(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}'`)
	if _, err := g.DB.Exec(`INSERT INTO combos VALUES('c','bundle','llm','["openai/model"]',NULL,'now','now');`); err != nil {
		t.Fatal(err)
	}
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
	})}}
	// "combo/bundle" é o id publicado pelo /v1/models e precisa rotear.
	w := httptest.NewRecorder()
	g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"combo/bundle","messages":[{"role":"user","content":"hi"}]}`)))
	if w.Code != 200 {
		t.Fatalf("id listado não roteia: %d %s", w.Code, w.Body.String())
	}
}

// A origem (open-sse/services/combo.js) devolve o status do PRIMEIRO candidato
// que falhou e a mensagem do ÚLTIMO (lastStatus/lastError). Antes disso, o
// combo zn-glm respondia 402 do Grok (fallback terminal) em vez do 429 real do
// GLM, escondendo a quota esgotada do modelo primário.
func TestComboAllFailedUsesFirstStatusAndLastMessage(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}'`)
	if _, err := g.DB.Exec(`INSERT INTO combos VALUES('c','bundle','llm','["openai/primary","openai/fallback"]',NULL,'now','now');`); err != nil {
		t.Fatal(err)
	}
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), `"model":"primary"`) {
			return &http.Response{StatusCode: 429, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"code":"1310","message":"Weekly/Monthly Limit Exhausted"}}`))}, nil
		}
		return &http.Response{StatusCode: 402, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":"Grok Build usage balance exhausted"}`))}, nil
	})}}
	w := httptest.NewRecorder()
	g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"bundle","messages":[{"role":"user","content":"hi"}]}`)))
	if w.Code != 429 {
		t.Fatalf("status %d, esperado 429: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Grok Build usage balance exhausted") {
		t.Fatalf("mensagem do último candidato ausente: %s", w.Body.String())
	}
}

// Quando um candidato bloqueia o fallback (400/422/499), a origem devolve a
// resposta DELE, não o status de uma falha anterior que já caiu para o próximo.
func TestComboBlockedCandidateReturnsItsOwnStatus(t *testing.T) {
	g := gatewayModelsFixture(t)
	g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}'`)
	if _, err := g.DB.Exec(`INSERT INTO combos VALUES('c','bundle','llm','["openai/primary","openai/blocked"]',NULL,'now','now');`); err != nil {
		t.Fatal(err)
	}
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(string(raw), `"model":"primary"`) {
			return &http.Response{StatusCode: 429, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":"rate limit"}`))}, nil
		}
		return &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"error":"context overflow"}`))}, nil
	})}}
	w := httptest.NewRecorder()
	g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"bundle","messages":[{"role":"user","content":"hi"}]}`)))
	if w.Code != 400 {
		t.Fatalf("status %d, esperado 400: %s", w.Code, w.Body.String())
	}
}

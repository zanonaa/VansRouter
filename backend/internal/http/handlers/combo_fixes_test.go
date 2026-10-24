package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

// Correções achadas no teste real dos combos com backup do usuário
// (2026-10-23): default stream, HTML upstream, combo aninhado e thinking
// em resposta Claude.

func TestClaudeThinkingResponseBecomesReasoningContent(t *testing.T) {
	native := map[string]any{
		"id": "msg_1", "type": "message", "role": "assistant", "model": "glm-5.3-flash",
		"content": []any{
			map[string]any{"type": "thinking", "thinking": "pensando..."},
			map[string]any{"type": "text", "text": "resposta"},
		},
		"stop_reason": "end_turn",
		"usage":       map[string]any{"input_tokens": 3.0, "output_tokens": 5.0},
	}
	out, err := convertChatResponse(native, "claude", "openai", "glm-5.3-flash")
	if err != nil {
		t.Fatal(err)
	}
	choice := out["choices"].([]any)[0].(map[string]any)
	message := choice["message"].(map[string]any)
	if message["content"] != "resposta" {
		t.Fatalf("content %v", message["content"])
	}
	if message["reasoning_content"] != "pensando..." {
		t.Fatalf("reasoning %v", message["reasoning_content"])
	}
}

func TestUpstreamHTMLIsRejectedEvenOn200(t *testing.T) {
	// A barreira de HTML vive no translator_send; o teste reproduz a decisão
	// por meio do helper isHTMLContentType extraído para ser testável.
	for _, test := range []struct {
		contentType string
		rejected    bool
	}{
		{"text/html; charset=utf-8", true},
		{"TEXT/HTML", true},
		{"text/event-stream", false},
		{"application/json", false},
		{"", false},
	} {
		if got := isHTMLContentType(test.contentType); got != test.rejected {
			t.Fatalf("%q -> %v, esperado %v", test.contentType, got, test.rejected)
		}
	}
}

func TestComboFallbackPolicy(t *testing.T) {
	// Fiel a accountFallback.js: 401/402/403/404/429/5xx caem para o próximo
	// model; os demais 4xx (400/408/410/422/499...) e bloqueio de conteúdo
	// param no primeiro.
	for _, test := range []struct {
		status int
		text   string
		want   bool
	}{
		{403, "FreeTierError", true},
		{401, "unauthorized", true},
		{404, "model not found", true},
		{429, "rate limit", true},
		{502, "bad gateway", true},
		{400, "context overflow", false},
		{408, "request timeout", false},
		{410, "model gone", false},
		{422, "invalid", false},
		{499, "client closed", false},
		{403, "content-blocked", false},
		{400, "Sensitive Content detected", false},
	} {
		if got := comboShouldFallback(test.status, test.text); got != test.want {
			t.Fatalf("status=%d text=%q -> %v, esperado %v", test.status, test.text, got, test.want)
		}
	}
}

func TestComboNestedReferenceExpandsWithoutSlash(t *testing.T) {
	// Expansão de referência aninhada é feita por expandComboEntries; a
	// entrada sem barra que não é combo passa adiante (comportamento origem).
	db := comboTestDB(t)
	if err := insertCombo(db, "zn-gpt", `["glm/glm-5.3"]`); err != nil {
		t.Fatal(err)
	}
	if err := insertCombo(db, "zn-glm", `["glm/glm-5.3", "zn-gpt"]`); err != nil {
		t.Fatal(err)
	}
	g := &Gateway{DB: db}
	models, err := g.expandComboEntries(httptest.NewRequest("POST", "/", nil), "zn-glm", []any{"glm/glm-5.3", "zn-gpt"})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(models)
	if string(encoded) != `["glm/glm-5.3","glm/glm-5.3"]` {
		t.Fatalf("models %s", encoded)
	}

	// Sem barra e sem combo correspondente: passa adiante para falhar
	// downstream como model (mesma semântica da origem).
	models, err = g.expandComboEntries(httptest.NewRequest("POST", "/", nil), "zn-glm", []any{"modelo-solto"})
	if err != nil || len(models) != 1 || models[0] != "modelo-solto" {
		t.Fatalf("solto: %v %v", models, err)
	}

	// Ciclo é rejeitado com erro explícito, não recursão infinita.
	if err := insertCombo(db, "zn-a", `["zn-b"]`); err != nil {
		t.Fatal(err)
	}
	if err := insertCombo(db, "zn-b", `["zn-a"]`); err != nil {
		t.Fatal(err)
	}
	if _, err := g.expandComboEntries(httptest.NewRequest("POST", "/", nil), "zn-a", []any{"zn-b"}); err == nil || !strings.Contains(err.Error(), "Circular") {
		t.Fatalf("ciclo: %v", err)
	}
}

func comboTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(context.Background(), dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := database.Migrate(context.Background(), db, dir); err != nil {
		t.Fatal(err)
	}
	return db
}

func insertCombo(db *sql.DB, name, modelsJSON string) error {
	_, err := db.Exec(`INSERT INTO combos(id,name,kind,models,context_length,createdAt,updatedAt) VALUES(?,?,NULL,?,NULL,'now','now')`, name, name, modelsJSON)
	return err
}

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

// recordingConnections registra o provider consultado e só devolve a conexão
// quando o id resolvido é o esperado (provider-node).
type recordingConnections struct {
	provider string
	expected string
	items    []map[string]any
}

func (r *recordingConnections) List(_ context.Context, provider string, _ *bool) ([]map[string]any, error) {
	r.provider = provider
	if provider != r.expected {
		return nil, nil
	}
	return r.items, nil
}
func (r *recordingConnections) Update(context.Context, string, map[string]any) (map[string]any, error) {
	return nil, nil
}
func (r *recordingConnections) DeleteByProvider(context.Context, string) (int64, error) {
	return 0, nil
}

func TestTranslatorSendAppliesUpstreamModelID(t *testing.T) {
	handler := &TranslatorSend{Connections: sendConnections{[]map[string]any{{"provider": "deepseek", "isActive": true, "apiKey": "secret"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		if err := json.Unmarshal(raw, &body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "deepseek-v4-pro" {
			t.Fatalf("id roteado vazou para o upstream: %v", body["model"])
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
	})}}
	w := httptest.NewRecorder()
	handler.Send(w, httptest.NewRequest("POST", "/", strings.NewReader(`{"provider":"deepseek","model":"deepseek-v4-pro-max","body":{"messages":[]}}`)))
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

func TestOpenAIChatResolvesProviderNodePrefix(t *testing.T) {
	g := gatewayModelsFixture(t)
	if _, err := g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}';
		CREATE TABLE providerNodes(id TEXT PRIMARY KEY,type TEXT,name TEXT,data TEXT,createdAt TEXT,updatedAt TEXT);
		INSERT INTO providerNodes VALUES('openai-compatible-chat-dahl','openai-compatible','Dahl','{"prefix":"da"}','now','now');`); err != nil {
		t.Fatal(err)
	}
	nodeID := "openai-compatible-chat-dahl"
	connections := &recordingConnections{expected: nodeID, items: []map[string]any{{
		"provider": nodeID, "isActive": true, "apiKey": "secret",
		"providerSpecificData": map[string]any{"baseUrl": "https://example.invalid/v1"},
	}}}
	g.Send = &TranslatorSend{Connections: connections, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
	})}}
	w := httptest.NewRecorder()
	g.OpenAIChat(w, httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"da/MiniMaxAI/MiniMax-M2.7","messages":[{"role":"user","content":"hi"}]}`)))
	// Antes da correção a conexão era buscada por "da" e a chamada morria com
	// "No active connection for provider: da".
	if connections.provider != nodeID {
		t.Fatalf("provider consultado: %q", connections.provider)
	}
	if w.Code != 200 {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
}

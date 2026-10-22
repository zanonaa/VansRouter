package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func TestChatHonorsRequiredKeyAndPermissions(t *testing.T) {
	g := gatewayModelsFixture(t)
	key, err := database.CreateApiKey(context.Background(), g.DB, "chat", "machine")
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	g.Send = &TranslatorSend{Connections: sendConnections{items: []map[string]any{{"isActive": true, "apiKey": "upstream"}}}, Client: &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("data: ok\n\n"))}, nil
	})}}
	invoke := func(token string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/v1/chat/completions", strings.NewReader(`{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hello"}]}`))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		g.OpenAIChat(w, r)
		return w
	}
	if w := invoke(""); w.Code != 401 || !strings.Contains(w.Body.String(), "Missing API key") {
		t.Fatalf("missing %d %s", w.Code, w.Body.String())
	}
	if w := invoke("invalid"); w.Code != 401 || !strings.Contains(w.Body.String(), "Invalid API key") {
		t.Fatalf("invalid %d %s", w.Code, w.Body.String())
	}
	for _, field := range []string{"allowedKinds", "allowedProviders"} {
		_, err := database.UpdateApiKey(context.Background(), g.DB, ttsString(key["id"]), map[string]any{field: []any{}})
		if err != nil {
			t.Fatal(err)
		}
		w := invoke(ttsString(key["key"]))
		if w.Code != 403 || calls != 0 {
			t.Fatalf("%s did not deny %d calls %d", field, w.Code, calls)
		}
		_, err = database.UpdateApiKey(context.Background(), g.DB, ttsString(key["id"]), map[string]any{field: nil})
		if err != nil {
			t.Fatal(err)
		}
	}
	w := invoke(ttsString(key["key"]))
	if w.Code != 200 || calls != 1 {
		t.Fatalf("permitted %d calls %d %s", w.Code, calls, w.Body.String())
	}
	if _, err := g.DB.Exec(`UPDATE settings SET data='{"requireApiKey":false}' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	w = invoke("")
	if w.Code != 200 || calls != 2 {
		t.Fatalf("local mode %d calls %d", w.Code, calls)
	}
}

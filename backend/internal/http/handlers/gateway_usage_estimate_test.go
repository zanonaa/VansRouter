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

// Regressão do dashboard vazio: providers OpenAI-compatible que não devolvem
// usage (ex.: free tiers) respondiam 200 mas não entravam em usageHistory.
// A origem Node estima body/4 (entrada) e conteúdo/4 (saída) nesse caso.
func TestGatewayEstimatesUsageWhenProviderOmitsIt(t *testing.T) {
	cases := []struct {
		name   string
		stream bool
		body   string
	}{
		{name: "stream", stream: true, body: "data: {\"choices\":[{\"delta\":{\"content\":\"hello world\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"},
		{name: "json", body: `{"choices":[{"message":{"content":"hello world"},"finish_reason":"stop"}]}`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx := context.Background()
			dir := t.TempDir()
			db, err := database.Open(ctx, dir, "NORMAL")
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err = database.Migrate(ctx, db, dir); err != nil {
				t.Fatal(err)
			}
			db.Exec(`INSERT INTO settings(id,data) VALUES(1,'{"requireApiKey":false}') ON CONFLICT(id) DO UPDATE SET data=excluded.data`)
			db.Exec(`INSERT INTO providerConnections(id,provider,authType,isActive,data,createdAt,updatedAt) VALUES('account','openai','apikey',1,'{"apiKey":"upstream"}','now','now')`)

			contentType := "application/json"
			if testCase.stream {
				contentType = "text/event-stream"
			}
			client := &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(testCase.body))}, nil
			})}
			g := &Gateway{DB: db, Send: &TranslatorSend{Connections: database.Connections{DB: db}, Client: client}}
			requestBody := `{"model":"openai/test","messages":[{"role":"user","content":"hello"}],"stream":false}`
			if testCase.stream {
				requestBody = strings.Replace(requestBody, `"stream":false`, `"stream":true`, 1)
			}
			w := httptest.NewRecorder()
			g.OpenAIChat(w, httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody)))
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}

			var count, prompt, completion int
			if err = db.QueryRow(`SELECT COUNT(*),COALESCE(MAX(promptTokens),0),COALESCE(MAX(completionTokens),0) FROM usageHistory`).Scan(&count, &prompt, &completion); err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Fatalf("usageHistory rows = %d, quer 1", count)
			}
			if prompt <= usageEstimateBuffer || completion != 2 {
				t.Fatalf("estimativa inesperada: prompt=%d completion=%d", prompt, completion)
			}

			stats, err := (&UsageContracts{DB: db, LiveUsage: &UsageLive{History: database.Usage{DB: db}, Connections: database.Connections{DB: db}}}).usageStats(httptest.NewRequest(http.MethodGet, "/api/usage/stats?period=today", nil), "today")
			if err != nil {
				t.Fatal(err)
			}
			if stats["totalRequests"] != float64(1) {
				t.Fatalf("totalRequests=%v", stats["totalRequests"])
			}
			if len(stats["byModel"].(map[string]any)) != 1 {
				t.Fatalf("byModel=%v", stats["byModel"])
			}
		})
	}
}

func TestUsageObserverCountsClaudeGeminiAndResponsesContent(t *testing.T) {
	cases := []struct {
		name  string
		event map[string]any
		want  int
	}{
		{name: "claude", event: map[string]any{"type": "content_block_delta", "delta": map[string]any{"text": "hello"}}, want: 5},
		{name: "gemini", event: map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": "hello"}}}}}}, want: 5},
		{name: "responses", event: map[string]any{"type": "response.output_text.delta", "delta": "hello"}, want: 5},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			u := &requestUsage{}
			u.countStreamContent(testCase.event)
			if u.contentChars != testCase.want {
				t.Fatalf("contentChars=%d, quer %d", u.contentChars, testCase.want)
			}
		})
	}
}

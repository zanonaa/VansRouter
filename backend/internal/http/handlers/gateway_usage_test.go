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
)

func TestGatewayPersistsUsageAndDailyWithoutDoubleCounting(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(map[bool]string{false: "json", true: "stream"}[stream], func(t *testing.T) {
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
			db.Exec(`INSERT INTO combos(id,name,models,createdAt,updatedAt) VALUES('combo','bundle','["openai/first","openai/final"]','now','now')`)
			(database.Pricing{DB: db}).Update(ctx, map[string]any{"openai": map[string]any{"final": map[string]any{"input": 2.0, "output": 4.0, "cached": 1.0}}})
			live := &UsageLive{History: database.Usage{DB: db}, Connections: database.Connections{DB: db}}
			calls := 0
			client := &http.Client{Transport: kiroImportTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return &http.Response{StatusCode: 429, Body: io.NopCloser(strings.NewReader(`{"error":"limited"}`))}, nil
				}
				snapshot, err := live.Snapshot(ctx)
				if err != nil {
					t.Fatal(err)
				}
				active := snapshot["activeRequests"].([]map[string]any)
				if len(active) != 1 {
					t.Fatalf("pending: %v", snapshot)
				}
				body := `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":100,"completion_tokens":20,"prompt_tokens_details":{"cached_tokens":10}}}`
				ct := "application/json"
				if stream {
					ct = "text/event-stream"
					body = "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":20,\"prompt_tokens_details\":{\"cached_tokens\":10}}}\n\ndata: [DONE]\n\n"
				}
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{ct}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			g := &Gateway{DB: db, LiveUsage: live, Send: &TranslatorSend{Connections: database.Connections{DB: db}, Client: client}}
			body := `{"model":"bundle","messages":[{"role":"user","content":"hello"}],"stream":false}`
			if stream {
				body = strings.Replace(body, `false`, `true`, 1)
			}
			w := httptest.NewRecorder()
			g.OpenAIChat(w, httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body)))
			if w.Code != 200 {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			var count, prompt, completion int
			var provider, model, connection string
			var cost float64
			if err = db.QueryRow(`SELECT COUNT(*),provider,model,connectionId,promptTokens,completionTokens,cost FROM usageHistory`).Scan(&count, &provider, &model, &connection, &prompt, &completion, &cost); err != nil {
				t.Fatal(err)
			}
			if count != 1 || provider != "openai" || model != "final" || connection != "account" || prompt != 100 || completion != 20 || cost < 0.000269 || cost > 0.000271 {
				t.Fatalf("history: %d %s %s %s %d %d %f", count, provider, model, connection, prompt, completion, cost)
			}
			var raw string
			if err = db.QueryRow(`SELECT data FROM usageDaily`).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var day map[string]any
			json.Unmarshal([]byte(raw), &day)
			if day["requests"] != float64(1) || day["promptTokens"] != float64(100) || day["byAccount"] == nil {
				t.Fatalf("daily: %v", day)
			}
			for _, period := range []string{"today", "7d", "all"} {
				stats, err := (&UsageContracts{DB: db, LiveUsage: live}).usageStats(httptest.NewRequest("GET", "/api/usage/stats", nil), period)
				if err != nil {
					t.Fatal(err)
				}
				if stats["totalRequests"] != float64(1) || len(stats["byModel"].(map[string]any)) != 1 || len(stats["byAccount"].(map[string]any)) != 1 {
					t.Fatalf("stats %s: %v", period, stats)
				}
			}
			snapshot, _ := live.Snapshot(ctx)
			if len(snapshot["activeRequests"].([]map[string]any)) != 0 || len(snapshot["recentRequests"].([]map[string]any)) != 1 {
				t.Fatalf("live: %v", snapshot)
			}
		})
	}
}

func TestUsageObserverHeaderlessSSEAndNativeDetails(t *testing.T) {
	for _, prefix := range []string{"event: response.created\n", ": keepalive\n\nevent: response.created\n"} {
		u := &requestUsage{}
		raw := prefix + "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":100,\"output_tokens\":20,\"input_tokens_details\":{\"cached_tokens\":80},\"output_tokens_details\":{\"reasoning_tokens\":7}}}}\n\n"
		body := &usageBody{ReadCloser: io.NopCloser(strings.NewReader(raw)), u: u}
		buffer := make([]byte, 2) // Split the event: prefix across reads.
		for {
			_, err := body.Read(buffer)
			if err != nil {
				break
			}
		}
		for key, want := range map[string]float64{"prompt_tokens": 100, "completion_tokens": 20, "cached_tokens": 80, "reasoning_tokens": 7} {
			if pxpipeNum(u.tokens[key]) != want {
				t.Fatalf("%s: got %v want %v", key, u.tokens, want)
			}
		}
	}
}

func TestUsageObserverGeminiParity(t *testing.T) {
	for _, candidates := range []float64{0, 12} {
		u := &requestUsage{}
		u.observe(map[string]any{"usageMetadata": map[string]any{"promptTokenCount": float64(100), "candidatesTokenCount": candidates, "thoughtsTokenCount": float64(8), "totalTokenCount": float64(120), "cachedContentTokenCount": float64(60)}})
		if pxpipeNum(u.tokens["completion_tokens"]) != 20 || pxpipeNum(u.tokens["reasoning_tokens"]) != 8 || pxpipeNum(u.tokens["cached_tokens"]) != 60 {
			t.Fatal(u.tokens)
		}
	}
}

func TestUsageObserverClaudePartialUsageAndPrettyJSON(t *testing.T) {
	u := &requestUsage{}
	body := &usageBody{ReadCloser: io.NopCloser(strings.NewReader("{\n\"usage\": {\"input_tokens\": 5,\"output_tokens\": 2,\"cache_read_input_tokens\":10}\n}")), u: u}
	io.ReadAll(body)
	u.observe(map[string]any{"usage": map[string]any{"output_tokens": float64(7)}})
	if pxpipeNum(u.tokens["prompt_tokens"]) != 15 || pxpipeNum(u.tokens["completion_tokens"]) != 7 {
		t.Fatal(u.tokens)
	}
}

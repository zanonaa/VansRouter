package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

// Contrato consumido por src/shared/components/UsageStats.js:
//
//	stats.totalRequests/totalPromptTokens/totalCompletionTokens/
//	totalCachedTokens/totalCost       -> number
//	stats.byProvider/byModel/byAccount/byApiKey/byEndpoint -> object
//	stats.pending.byModel/byAccount   -> object
//	stats.activeRequests/recentRequests/last10Minutes      -> array
//	stats.errorProvider               -> string
//
// Cada item dos by* precisa de requests/promptTokens/completionTokens/
// cachedTokens/cost numéricos; byModel/byAccount/byApiKey/byEndpoint também
// expõem rawModel/provider e a chave específica usada na tabela.
func TestUsageStatsJSONContractMatchesUsageStatsComponent(t *testing.T) {
	t.Run("banco vazio", func(t *testing.T) {
		db := openUsageContractDB(t)
		payload := requestUsageStats(t, db, "today")
		assertUsageContract(t, payload)
		assertUsageTotals(t, payload, 0)
		for _, group := range usageGroups {
			if len(payload[group].(map[string]any)) != 0 {
				t.Fatalf("%s deveria estar vazio: %v", group, payload[group])
			}
		}
	})

	t.Run("banco com dados", func(t *testing.T) {
		db := openUsageContractDB(t)
		ctx := context.Background()
		if err := (database.Usage{DB: db}).RecordUsage(ctx, map[string]any{
			"provider":     "openai",
			"model":        "gpt-contract",
			"connectionId": "account",
			"apiKey":       "sk-contract-1234567890",
			"endpoint":     "/v1/chat/completions",
			"tokens":       map[string]any{"prompt_tokens": float64(11), "completion_tokens": float64(7), "cached_tokens": float64(2)},
			"cost":         float64(0.25),
		}); err != nil {
			t.Fatal(err)
		}
		payload := requestUsageStats(t, db, "today")
		assertUsageContract(t, payload)
		assertUsageTotals(t, payload, 1)
		if payload["totalPromptTokens"] != float64(11) || payload["totalCompletionTokens"] != float64(7) || payload["totalCachedTokens"] != float64(2) || payload["totalCost"] != float64(0.25) {
			t.Fatalf("totais: %v", payload)
		}
		modelEntry := payload["byModel"].(map[string]any)["gpt-contract (openai)"].(map[string]any)
		if modelEntry["rawModel"] != "gpt-contract" || modelEntry["provider"] != "openai" || modelEntry["requests"] != float64(1) {
			t.Fatalf("byModel: %v", modelEntry)
		}
		accountEntry := payload["byAccount"].(map[string]any)["gpt-contract (openai - account)"].(map[string]any)
		if accountEntry["accountName"] != "account" || accountEntry["connectionId"] != "account" {
			t.Fatalf("byAccount: %v", accountEntry)
		}
		keyEntry := payload["byApiKey"].(map[string]any)["sk-contr***7890|gpt-contract|openai"].(map[string]any)
		if keyEntry["keyName"] != "sk-contr***7890" || keyEntry["apiKeyKey"] != "sk-contr***7890" {
			t.Fatalf("byApiKey: %v", keyEntry)
		}
		endpointEntry := payload["byEndpoint"].(map[string]any)["/v1/chat/completions|gpt-contract|openai"].(map[string]any)
		if endpointEntry["endpoint"] != "/v1/chat/completions" {
			t.Fatalf("byEndpoint: %v", endpointEntry)
		}
		if len(payload["recentRequests"].([]any)) != 1 {
			t.Fatalf("recentRequests: %v", payload["recentRequests"])
		}
	})

	t.Run("período sem dados", func(t *testing.T) {
		db := openUsageContractDB(t)
		old := time.Now().UTC().AddDate(0, 0, -10)
		day := old.Format("2006-01-02")
		stamp := old.Format("2006-01-02T15:04:05.000Z")
		if _, err := db.Exec(`INSERT INTO usageHistory(timestamp,provider,model,connectionId,apiKey,endpoint,promptTokens,completionTokens,cost,status,tokens) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
			stamp, "openai", "old-model", "account", "sk-old-1234567890", "/v1/chat/completions", 5, 3, 0.01, "ok", `{"prompt_tokens":5,"completion_tokens":3}`); err != nil {
			t.Fatal(err)
		}
		data := `{"requests":1,"promptTokens":5,"completionTokens":3,"cachedTokens":0,"cost":0.01,` +
			`"byProvider":{"openai":{"requests":1,"promptTokens":5,"completionTokens":3,"cachedTokens":0,"cost":0.01}},` +
			`"byModel":{"old-model|openai":{"requests":1,"promptTokens":5,"completionTokens":3,"cachedTokens":0,"cost":0.01,"rawModel":"old-model","provider":"openai"}},` +
			`"byAccount":{"account":{"requests":1,"promptTokens":5,"completionTokens":3,"cachedTokens":0,"cost":0.01,"rawModel":"old-model","provider":"openai"}},` +
			`"byApiKey":{"sk-old-1234567890|old-model|openai":{"requests":1,"promptTokens":5,"completionTokens":3,"cachedTokens":0,"cost":0.01,"rawModel":"old-model","provider":"openai","apiKey":"sk-old-1234567890"}},` +
			`"byEndpoint":{"/v1/chat/completions|old-model|openai":{"requests":1,"promptTokens":5,"completionTokens":3,"cachedTokens":0,"cost":0.01,"rawModel":"old-model","provider":"openai","endpoint":"/v1/chat/completions"}}}`
		if _, err := db.Exec(`INSERT INTO usageDaily(dateKey,data) VALUES(?,?)`, day, data); err != nil {
			t.Fatal(err)
		}

		empty := requestUsageStats(t, db, "7d")
		assertUsageContract(t, empty)
		assertUsageTotals(t, empty, 0)
		for _, group := range usageGroups {
			if len(empty[group].(map[string]any)) != 0 {
				t.Fatalf("7d deveria ignorar %s: %v", group, empty[group])
			}
		}

		filled := requestUsageStats(t, db, "60d")
		assertUsageContract(t, filled)
		assertUsageTotals(t, filled, 1)
		if _, ok := filled["byModel"].(map[string]any)["old-model (openai)"]; !ok {
			t.Fatalf("60d byModel: %v", filled["byModel"])
		}
	})
}

var usageGroups = []string{"byProvider", "byModel", "byAccount", "byApiKey", "byEndpoint"}

func openUsageContractDB(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	dir := t.TempDir()
	db, err := database.Open(ctx, dir, "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = database.Migrate(ctx, db, dir); err != nil {
		t.Fatal(err)
	}
	return db
}

func requestUsageStats(t *testing.T, db *sql.DB, period string) map[string]any {
	t.Helper()
	handler := &UsageContracts{DB: db, Connections: database.Connections{DB: db}, LiveUsage: &UsageLive{History: database.Usage{DB: db}, Connections: database.Connections{DB: db}}}
	recorder := httptest.NewRecorder()
	handler.Stats(recorder, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/usage/stats?period=%s", period), nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func assertUsageTotals(t *testing.T, payload map[string]any, requests float64) {
	t.Helper()
	if payload["totalRequests"] != requests {
		t.Fatalf("totalRequests=%v, quer %v", payload["totalRequests"], requests)
	}
}

func assertUsageContract(t *testing.T, payload map[string]any) {
	t.Helper()
	if payload["period"] == nil {
		t.Fatal("period ausente")
	}
	for _, field := range []string{"totalRequests", "totalPromptTokens", "totalCompletionTokens", "totalCachedTokens", "totalCost"} {
		if _, ok := payload[field].(float64); !ok {
			t.Fatalf("%s deveria ser number: %T", field, payload[field])
		}
	}
	for _, field := range usageGroups {
		group, ok := payload[field].(map[string]any)
		if !ok {
			t.Fatalf("%s deveria ser object: %T", field, payload[field])
		}
		for key, raw := range group {
			entry, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("%s[%s] deveria ser object", field, key)
			}
			for _, number := range []string{"requests", "promptTokens", "completionTokens", "cachedTokens", "cost"} {
				if _, ok := entry[number].(float64); !ok {
					t.Fatalf("%s[%s].%s deveria ser number: %T", field, key, number, entry[number])
				}
			}
		}
	}
	for _, field := range []string{"activeRequests", "recentRequests", "last10Minutes"} {
		if _, ok := payload[field].([]any); !ok {
			t.Fatalf("%s deveria ser array: %T", field, payload[field])
		}
	}
	pending, ok := payload["pending"].(map[string]any)
	if !ok {
		t.Fatalf("pending deveria ser object: %T", payload["pending"])
	}
	for _, field := range []string{"byModel", "byAccount"} {
		if _, ok := pending[field].(map[string]any); !ok {
			t.Fatalf("pending.%s deveria ser object: %T", field, pending[field])
		}
	}
	if _, ok := payload["errorProvider"].(string); !ok {
		t.Fatalf("errorProvider deveria ser string: %T", payload["errorProvider"])
	}
}

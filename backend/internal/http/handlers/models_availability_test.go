package handlers

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type availabilityStub struct {
	items   []map[string]any
	updates []string
	patches map[string]map[string]any
}

func (s *availabilityStub) List(_ context.Context, provider string, active *bool) ([]map[string]any, error) {
	result := []map[string]any{}
	for _, item := range s.items {
		if provider != "" && item["provider"] != provider {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *availabilityStub) Update(_ context.Context, id string, patch map[string]any) (map[string]any, error) {
	s.updates = append(s.updates, id)
	if s.patches == nil {
		s.patches = map[string]map[string]any{}
	}
	s.patches[id] = patch
	return patch, nil
}

func (s *availabilityStub) DeleteByProvider(context.Context, string) (int64, error) { return 0, nil }

func TestAvailabilityGet(t *testing.T) {
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	past := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	store := &availabilityStub{items: []map[string]any{
		{"id": "c1", "provider": "claude", "name": "Trabalho", "modelLock_google/gemini": future},
		{"id": "c2", "provider": "codex", "email": "contato@x", "testStatus": "unavailable", "lastError": "boom", "modelLock_bad": past},
		{"id": "c3", "provider": "glm"},
	}}
	handler := &ModelAvailability{Connections: store}
	w := httptest.NewRecorder()
	handler.Get(w, httptest.NewRequest("GET", "/api/models/availability", nil))
	if w.Code != 200 {
		t.Fatalf("get %d %s", w.Code, w.Body.String())
	}
	var payload struct {
		Models []map[string]any `json:"models"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Models) != 2 {
		t.Fatalf("models %v", payload.Models)
	}
	id, codexModel := payload.Models[0], payload.Models[1]
	// key.slice(MODEL_LOCK_PREFIX.length): "modelLock_google/gemini" → "google/gemini".
	if id["provider"] != "claude" || id["status"] != "cooldown" || id["model"] != "google/gemini" {
		t.Fatalf("locks %v", id)
	}
	if id["until"] != future || id["connectionName"] != "Trabalho" {
		t.Fatalf("lock detail %v", id)
	}
	_ = codexModel
	if codexModel["model"] != "__all" || codexModel["status"] != "unavailable" || codexModel["connectionName"] != "contato@x" || codexModel["lastError"] != "boom" {
		t.Fatalf("codex model %v", codexModel)
	}
}

func TestAvailabilityPost(t *testing.T) {
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	store := &availabilityStub{items: []map[string]any{
		{"id": "c1", "provider": "claude", "modelLock_bad-model": future, "testStatus": "unavailable"},
		{"id": "c2", "provider": "claude", "modelLock_other-model": future},
	}}
	handler := &ModelAvailability{Connections: store}
	body := `{"action":"clearCooldown","provider":"claude","model":"bad-model"}`
	w := httptest.NewRecorder()
	handler.Post(w, httptest.NewRequest("POST", "/api/models/availability", strings.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("post %d %s", w.Code, w.Body.String())
	}
	var response map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	if response["ok"] != true {
		t.Fatalf("response %v", response)
	}
	if len(store.updates) != 1 || store.updates[0] != "c1" {
		t.Fatalf("updates %v", store.updates)
	}
	patch := store.patches["c1"]
	if patch["modelLock_bad-model"] != nil {
		t.Fatalf("lock nil %v", patch)
	}
	if patch["testStatus"] != "active" || patch["lastError"] != nil || patch["lastErrorAt"] != nil || pxpipeNum(patch["backoffLevel"]) != 0 {
		t.Fatalf("patch %v", patch)
	}
	// action/provider/model inválido → 400 literal.
	w = httptest.NewRecorder()
	handler.Post(w, httptest.NewRequest("POST", "/api/models/availability", strings.NewReader(`{"action":"none"}`)))
	if w.Code != 400 {
		t.Fatalf("invalid %d", w.Code)
	}
	var failed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if failed["error"] != "Invalid request" {
		t.Fatalf("error %v", failed)
	}
}

func TestModelCatalogSyncGet(t *testing.T) {
	dataDir := t.TempDir()
	syncer := &ModelCatalogSync{DataDir: dataDir}
	// Sem arquivo: catalog null.
	w := httptest.NewRecorder()
	syncer.Get(w, httptest.NewRequest("GET", "/api/models/catalog-sync", nil))
	if w.Code != 200 {
		t.Fatalf("get %d", w.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["catalog"] != nil {
		t.Fatalf("catalog %v", payload["catalog"])
	}
	if payload["running"] != false || payload["url"] != "https://models.dev/api.json" || payload["intervalMs"] != float64(86400000) {
		t.Fatalf("state %v", payload)
	}
	// Com arquivo: resumo com contagens.
	now := time.Now().UnixMilli()
	catalogContent := `{"v":2,"etag":"\"abc\"","syncedAt":` + int64String(now) + `,"models":{"glm:glm-4.6":{}},"providers":{"glm":{}}}`
	if err := os.WriteFile(catalogFile(dataDir), []byte(catalogContent), 0o644); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	syncer.Get(w, httptest.NewRequest("GET", "/api/models/catalog-sync", nil))
	payload = map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	summary, _ := payload["catalog"].(map[string]any)
	if summary == nil || summary["models"] != float64(1) || summary["providers"] != float64(1) || summary["etag"] != nil {
		t.Fatalf("summary %v", summary)
	}
	if summary["bytes"] != float64(len(catalogContent)) {
		t.Fatalf("bytes %v", summary["bytes"])
	}
}

func TestModelCatalogSyncPost(t *testing.T) {
	dataDir := t.TempDir()
	upstream := map[string]any{
		"zai": map[string]any{
			"models": map[string]any{
				"glm-4.6v:free": map[string]any{
					"modalities": map[string]any{"input": []any{"text", "image"}},
					"limit":      map[string]any{"context": 131072, "output": 32768},
				},
			},
		},
	}
	entries := func() []map[string]any {
		return []map[string]any{{
			"provider":      "glm",
			"model":         "glm-4.6v",
			"contextLength": float64(0),
			"current":       map[string]any{"contextWindow": 131072, "maxOutput": 65536},
		}}
	}
	syncer := &ModelCatalogSync{
		DataDir: dataDir,
		Fetch: func(ctx context.Context, ifNoneMatch string) (int, string, []byte, error) {
			if ifNoneMatch != "" {
				t.Fatalf("if-none-match %q em primeira corrida", ifNoneMatch)
			}
			encoded, _ := json.Marshal(upstream)
			return 200, `"v1"`, encoded, nil
		},
		Entries: entries,
	}
	w := httptest.NewRecorder()
	syncer.Post(w, httptest.NewRequest("POST", "/api/models/catalog-sync", nil))
	if w.Code != 200 {
		t.Fatalf("post %d %s", w.Code, w.Body.String())
	}
	var payload struct {
		Success bool `json:"success"`
		Result  struct {
			Status    string `json:"status"`
			Etag      string `json:"etag"`
			Bytes     int    `json:"bytes"`
			Models    int    `json:"models"`
			Providers int    `json:"providers"`
		} `json:"result"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Success || payload.Result.Status != "updated" || payload.Result.Etag != `"v1"` {
		t.Fatalf("result %+v", payload)
	}
	// O delta de maxOutput entra (65536 → 32768) e contextWindow fica (131072 igual).
	raw, err := os.ReadFile(catalogFile(dataDir))
	if err != nil {
		t.Fatal(err)
	}
	var written struct {
		V         int            `json:"v"`
		Etag      string         `json:"etag"`
		Models    map[string]any `json:"models"`
		Providers map[string]any `json:"providers"`
	}
	if err := json.Unmarshal(raw, &written); err != nil {
		t.Fatal(err)
	}
	if written.V != 2 || written.Etag != `"v1"` {
		t.Fatalf("header %d %s", written.V, written.Etag)
	}
	// Modalidades filed under local+upstream
	if written.Models["glm:glm-4.6v"] == nil || written.Models["zai:glm-4.6v"] == nil {
		t.Fatalf("models %+v", written.Models)
	}
	providersBlock := written.Providers["glm"].(map[string]any)
	delta, _ := providersBlock["glm-4.6v"].(map[string]any)
	if delta["maxOutput"] != float64(32768) || delta["contextWindow"] != nil {
		t.Fatalf("delta %v", delta)
	}
	// Segunda corrida: forall-none-match + 304 → unchanged.
	var seenIfNoneMatch string
	syncer.Fetch = func(ctx context.Context, ifNoneMatch string) (int, string, []byte, error) {
		seenIfNoneMatch = ifNoneMatch
		return 304, "", nil, nil
	}
	w = httptest.NewRecorder()
	syncer.Post(w, httptest.NewRequest("POST", "/api/models/catalog-sync", nil))
	payload2 := struct {
		Success bool `json:"success"`
		Result  struct {
			Status string `json:"status"`
		} `json:"result"`
	}{}
	_ = json.Unmarshal(w.Body.Bytes(), &payload2)
	if payload2.Result.Status != "unchanged" || seenIfNoneMatch != `"v1"` {
		t.Fatalf("second %+v inm=%q", payload2, seenIfNoneMatch)
	}
}

func TestModelCatalogSyncErrors(t *testing.T) {
	dataDir := t.TempDir()
	syncer := &ModelCatalogSync{
		DataDir: dataDir,
		Fetch: func(ctx context.Context, ifNoneMatch string) (int, string, []byte, error) {
			return 500, "", nil, nil
		},
	}
	w := httptest.NewRecorder()
	syncer.Post(w, httptest.NewRequest("POST", "/api/models/catalog-sync", nil))
	if w.Code != 503 {
		t.Fatalf("status %d %s", w.Code, w.Body.String())
	}
	var failed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if failed["error"] != "HTTP 500" {
		t.Fatalf("error %v", failed)
	}
	// running true → POST devolve 503 "sync in progress".
	syncer.state.mu.Lock()
	syncer.state.running = true
	syncer.state.mu.Unlock()
	w = httptest.NewRecorder()
	syncer.Post(w, httptest.NewRequest("POST", "/api/models/catalog-sync", nil))
	failed = map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if failed["error"] != "sync in progress" {
		t.Fatalf("running %v", failed)
	}
	syncer.state.mu.Lock()
	syncer.state.running = false
	syncer.state.mu.Unlock()
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}

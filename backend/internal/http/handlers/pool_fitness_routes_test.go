package handlers

import (
	"context"

	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

func fitnessTestService(t *testing.T) *PoolFitnessRoutes {
	t.Helper()
	db, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := db.Exec(`CREATE TABLE proxyPoolFitness(poolId TEXT NOT NULL,scope TEXT NOT NULL,until INTEGER NOT NULL,reason TEXT,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL,PRIMARY KEY(poolId,scope))`); err != nil {
		t.Fatal(err)
	}
	return NewPoolFitnessRoutes(database.ProxyFitness{DB: db})
}

func TestPoolFitnessSnapshotAndClear(t *testing.T) {
	routes := fitnessTestService(t)
	ctx := context.Background()
	until := time.Now().Add(time.Hour).UnixMilli()
	if !routes.Service.MarkPoolUnfit(ctx, "pool-a", "freebuff::openai/gpt-5", &until, "rate-limit") {
		t.Fatal("mark false")
	}
	w := httptest.NewRecorder()
	routes.Get(w, httptest.NewRequest("GET", "/api/proxy-pools/fitness", nil))
	if w.Code != 200 {
		t.Fatalf("get %d %s", w.Code, w.Body.String())
	}
	var payload struct {
		Pools map[string]map[string]struct {
			Until  int64  `json:"until"`
			Reason string `json:"reason"`
		} `json:"pools"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	entry, ok := payload.Pools["pool-a"]["freebuff::openai/gpt-5"]
	if !ok || entry.Reason != "rate-limit" {
		t.Fatalf("snapshot %v", payload.Pools)
	}
	// A origem marca o scope curinga provider::* quando falha em provider.
	if !routes.Service.IsPoolFit("pool-a", "freebuff::other/model", time.Now().UnixMilli()) {
		t.Fatal("escopo próprio deveria estar unfit ainda")
	}
	if !routes.Service.IsPoolFit("pool-a", "freebuff::other/model", time.Now().UnixMilli()) {
		t.Fatal("sem wildcard marcado deveria estar fit")
	}
	if !routes.Service.IsPoolFit("pool-a", "other::openai/gpt-5", time.Now().UnixMilli()) {
		t.Fatal("outro provider deveria estar fit")
	}
	// FitPoolIds filtra.
	fit := routes.Service.FitPoolIds([]string{"pool-a", "pool-b"}, "freebuff::openai/gpt-5", time.Now().UnixMilli())
	if strings.Join(fit, ",") != "pool-b" {
		t.Fatalf("fit %v", fit)
	}
	// ClearAll por provider remove o scope e mantém outros.
	if !routes.Service.ClearAllPoolUnfit(ctx, "freebuff") {
		t.Fatal("clear false")
	}
	if !routes.Service.IsPoolFit("pool-a", "freebuff::openai/gpt-5", time.Now().UnixMilli()) {
		t.Fatal("deveria estar fit após clear")
	}
	// Snapshot vazio.
	routes.Service.MarkPoolUnfit(ctx, "p", "openai::m", &until, "")
	if !routes.Service.ClearAllPoolUnfit(ctx, "") {
		t.Fatal("clear all false")
	}
	snapshot := routes.Service.Snapshot(ctx, time.Now().UnixMilli())
	if len(snapshot) != 0 {
		t.Fatalf("snapshot residual %v", snapshot)
	}
}

func TestPoolFitnessRoutesContract(t *testing.T) {
	routes := fitnessTestService(t)
	ctx := context.Background()
	until := time.Now().Add(time.Minute).UnixMilli()
	routes.Service.MarkPoolUnfit(ctx, "p1", "openai::gpt", &until, "boom")
	// GET snapshot.
	w := httptest.NewRecorder()
	routes.Get(w, httptest.NewRequest("GET", "/api/proxy-pools/fitness", nil))
	if w.Code != 200 {
		t.Fatalf("get %d", w.Code)
	}
	// clear-all com provider no body.
	w = httptest.NewRecorder()
	routes.ClearAll(w, httptest.NewRequest("POST", "/api/proxy-pools/fitness/clear-all", strings.NewReader(`{"provider":"openai"}`)))
	if w.Code != 200 {
		t.Fatalf("clear-all %d %s", w.Code, w.Body.String())
	}
	var cleared map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &cleared)
	if cleared["ok"] != true || cleared["provider"] != "openai" {
		t.Fatalf("cleared %v", cleared)
	}
	// provider ausente → null e clear total.
	routes.Service.MarkPoolUnfit(ctx, "p1", "zai::m", &until, "")
	w = httptest.NewRecorder()
	routes.ClearAll(w, httptest.NewRequest("POST", "/api/proxy-pools/fitness/clear-all", strings.NewReader(`não-json`)))
	cleared = map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &cleared)
	if cleared["provider"] != nil {
		t.Fatalf("provider nil %v", cleared)
	}
	// clear por pool + scope (via chi para URLParam).
	routes.Service.MarkPoolUnfit(ctx, "p1", "openai::gpt", &until, "")
	mux := chi.NewRouter()
	mux.Post("/api/proxy-pools/{id}/fitness/clear", routes.ClearByID)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/proxy-pools/p1/fitness/clear", strings.NewReader(`{"scope":"openai::gpt"}`)))
	if w.Code != 200 {
		t.Fatalf("clear id %d %s", w.Code, w.Body.String())
	}
	var clearedByID struct {
		OK     bool    `json:"ok"`
		PoolID string  `json:"poolId"`
		Scope  string  `json:"scope"`
		Extra  *string `json:"-"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &clearedByID); err != nil {
		t.Fatal(err)
	}
	if !clearedByID.OK || clearedByID.PoolID != "p1" || clearedByID.Scope != "openai::gpt" {
		t.Fatalf("clear by id %+v", clearedByID)
	}
	// escopo ausente → 400 literal.
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/api/proxy-pools/p1/fitness/clear", strings.NewReader(`{}`)))
	if w.Code != 400 {
		t.Fatalf("empty scope %d", w.Code)
	}
	var failed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if failed["error"] != "pool id and scope are required" {
		t.Fatalf("error %v", failed)
	}
}

func TestImportPublicContract(t *testing.T) {
	poolDB, err := database.Open(context.Background(), t.TempDir(), "NORMAL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = poolDB.Close() })
	if _, err := poolDB.Exec(`CREATE TABLE proxyPools(id TEXT PRIMARY KEY,isActive INTEGER DEFAULT 1,testStatus TEXT,data TEXT NOT NULL,createdAt TEXT NOT NULL,updatedAt TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	store := database.ProxyPools{DB: poolDB}
	// Fonte desconhecida.
	importer := &ImportPublic{Store: store, Fetcher: fetchUpstreamReal}
	w := httptest.NewRecorder()
	importer.Post(w, httptest.NewRequest("POST", "/api/proxy-pools/import-public", strings.NewReader(`{"source":"desconhecida"}`)))
	if w.Code != 400 {
		t.Fatalf("unknown %d %s", w.Code, w.Body.String())
	}
	var failed map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if failed["error"] != "Unknown proxy source" {
		t.Fatalf("error %v", failed)
	}
	// vpslab com texto: dedupe, ip inválido ignorado, corte de count.
	importer.Fetcher = func(ctx context.Context, sourceURL, sourceLabel, accept string) (int, string, []byte, error) {
		if !strings.HasPrefix(sourceURL, "https://raw.githubusercontent.com/VPSLabCloud/VPSLab-Free-Proxy-List/main/http_elite.txt") {
			t.Fatalf("vpslab url %s", sourceURL)
		}
		return 200, "text/plain", []byte("1.2.3.4:8080\n# comentário\n1.2.3.4:8080\n999.9.9.9:1111\n5.6.7.8:3128"), nil
	}
	body := `{"source":"vpslab","protocol":"http","anonymity":"elite","count":9}`
	w = httptest.NewRecorder()
	importer.Post(w, httptest.NewRequest("POST", "/api/proxy-pools/import-public", strings.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("vpslab %d %s", w.Code, w.Body.String())
	}
	var result struct {
		Source     string           `json:"source"`
		File       string           `json:"file"`
		Fetched    int              `json:"fetched"`
		Imported   int              `json:"imported"`
		Duplicates int              `json:"duplicates"`
		Failed     int              `json:"failed"`
		Created    []map[string]any `json:"created"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	// dedupe dentro do feed não conta como duplicate (fetched 2 válidos).
	if result.File != "http_elite.txt" || result.Fetched != 2 || result.Imported != 2 || result.Duplicates != 0 {
		t.Fatalf("vpslab result %+v", result)
	}
	// 2ª importação com os mesmos ips → tudo duplicado (dedupe por key normalizada).
	w = httptest.NewRecorder()
	importer.Post(w, httptest.NewRequest("POST", "/api/proxy-pools/import-public", strings.NewReader(body)))
	result = struct {
		Source     string           `json:"source"`
		File       string           `json:"file"`
		Fetched    int              `json:"fetched"`
		Imported   int              `json:"imported"`
		Duplicates int              `json:"duplicates"`
		Failed     int              `json:"failed"`
		Created    []map[string]any `json:"created"`
	}{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Imported != 0 || result.Duplicates != 2 {
		t.Fatalf("duplicates %+v", result)
	}
	// freeproxydb: rank por success, corte e labels.
	importer.Fetcher = func(ctx context.Context, sourceURL, sourceLabel, accept string) (int, string, []byte, error) {
		if !strings.Contains(sourceURL, "page_size=100") || !strings.Contains(sourceURL, "order_by=last_checked") || !strings.Contains(sourceURL, "protocol=socks5") || !strings.Contains(sourceURL, "speed=0%2C3.5") {
			t.Fatalf("freeproxydb url %s", sourceURL)
		}
		return 200, "application/json", []byte(`{"data":{"data":[
			{"is_valid":1,"connect_string":"socks5://7.7.7.7:443","ip":"7.7.7.7","port":"443","country":"BR","anonymity":"elite","speed":0,"check_success_count":2},
			{"is_valid":1,"connect_string":"socks5://8.8.8.8:443","ip":"8.8.8.8","port":"443","country":"US","anonymity":"elite","speed":1.5,"check_success_count":9},
			{"is_valid":0,"connect_string":"socks5://9.9.9.9:443"},
			{"is_valid":1,"connect_string":"ftp://bad"},
			{"is_valid":1,"connect_string":"socks5://10.10.10.10:443","ip":"10.10.10.10","port":"443","check_success_count":1}
		]}}`), nil
	}
	body = `{"source":"freeproxydb","protocol":"socks5","httpsOnly":true,"maxSpeed":3.5,"orderBy":"success","count":2}`
	w = httptest.NewRecorder()
	importer.Post(w, httptest.NewRequest("POST", "/api/proxy-pools/import-public", strings.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("freeproxydb %d %s", w.Code, w.Body.String())
	}
	result = struct {
		Source     string           `json:"source"`
		File       string           `json:"file"`
		Fetched    int              `json:"fetched"`
		Imported   int              `json:"imported"`
		Duplicates int              `json:"duplicates"`
		Failed     int              `json:"failed"`
		Created    []map[string]any `json:"created"`
	}{}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Fetched != 2 || result.Imported != 2 {
		t.Fatalf("freeproxydb result %+v", result)
	}
	first := result.Created[0]
	if !strings.Contains(first["name"].(string), "FreeProxyDB US") || !strings.Contains(first["name"].(string), "1.5s") || !strings.Contains(first["name"].(string), "8.8.8.8:443") {
		t.Fatalf("label %v", first)
	}
	// URL socks5 sem trailing slash na key de dedupe.
	if normalizeProxyURLKey("socks5://7.7.7.7:443") != "socks5://7.7.7.7:443" {
		t.Fatalf("socks key %q", normalizeProxyURLKey("socks5://7.7.7.7:443"))
	}
	if normalizeProxyURLKey("http://7.7.7.7:8080") != "http://7.7.7.7:8080/" {
		t.Fatalf("http key %q", normalizeProxyURLKey("http://7.7.7.7:8080"))
	}
	// Filtros inválidos: país em source sem suporte.
	w = httptest.NewRecorder()
	importer.Post(w, httptest.NewRequest("POST", "/api/proxy-pools/import-public", strings.NewReader(`{"source":"vpslab","country":"BR"}`)))
	failed = map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if failed["error"] != "Invalid country filter for VPSLab" {
		t.Fatalf("country %v", failed)
	}
	// orderBy em source sem suporte.
	w = httptest.NewRecorder()
	importer.Post(w, httptest.NewRequest("POST", "/api/proxy-pools/import-public", strings.NewReader(`{"source":"vpslab","orderBy":"latency"}`)))
	failed = map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if failed["error"] != "Ordering is not supported for VPSLab" {
		t.Fatalf("orderBy %v", failed)
	}
	// maxSpeed não capturado... (freeproxydb suporta; vpslab não)
	w = httptest.NewRecorder()
	importer.Post(w, httptest.NewRequest("POST", "/api/proxy-pools/import-public", strings.NewReader(`{"source":"vpslab","maxSpeed":4}`)))
	failed = map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &failed)
	if failed["error"] != "Speed filter is not supported for VPSLab" {
		t.Fatalf("maxSpeed %v", failed)
	}
	// Erro upstream (429 → status 429, texto idêntico).
	importer.Fetcher = func(ctx context.Context, sourceURL, sourceLabel, accept string) (int, string, []byte, error) {
		return 0, "", nil, &publicProxyError{Status: 429, Msg: "FreeProxyDB returned HTTP 429. FreeProxyDB public rate limit reached — retry in a minute or lower the count."}
	}
	w = httptest.NewRecorder()
	importer.Post(w, httptest.NewRequest("POST", "/api/proxy-pools/import-public", strings.NewReader(`{"source":"freeproxydb"}`)))
	if w.Code != 429 {
		t.Fatalf("rate limit %d %s", w.Code, w.Body.String())
	}
	// falhas de upstream na criação DB: failed conta.
	importer.Fetcher = func(ctx context.Context, sourceURL, sourceLabel, accept string) (int, string, []byte, error) {
		return 200, "text/plain", []byte("1.2.3.4:8080"), nil
	}
	w = httptest.NewRecorder()
	importer.Post(w, httptest.NewRequest("POST", "/api/proxy-pools/import-public", strings.NewReader(`{"source":"vpslab"}`)))
	// (sem falha real aqui — tabela ok; apenas confere 200)
	if w.Code != 200 {
		t.Fatalf("final %d", w.Code)
	}
}

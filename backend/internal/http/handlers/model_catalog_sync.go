package handlers

// Porta de src/lib/modelCatalog/sync.js + src/app/api/models/catalog-sync/route.js.
import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/providers"
)

const (
	modelCatalogURL      = "https://models.dev/api.json"
	modelCatalogVersion  = 2
	modelCatalogInterval = 24 * time.Hour
	modelCatalogStart    = 60 * time.Second
	modelCatalogRetry    = 30 * time.Minute
	modelCatalogFetchTO  = 60 * time.Second
)

// catalogAliases: id local -> id upstream no models.dev.
var catalogAliases = map[string]string{
	"github":        "github-copilot",
	"glm":           "zai",
	"glm-cn":        "zhipuai",
	"claude":        "anthropic",
	"gemini":        "google",
	"kimi":          "moonshotai",
	"kimi-cn":       "moonshotai-cn",
	"qwen":          "alibaba",
	"qwen-cn":       "alibaba-cn",
	"zhipu":         "zhipuai",
	"hunyuan":       "tencent",
	"doubao":        "volcengine",
	"cloudflare-ai": "cloudflare-workers-ai",
}

// Estado equivalente ao módulo JS (uma instância global do sync).
type modelCatalogSyncState struct {
	mu          sync.Mutex
	running     bool
	started     bool
	lastSync    *int64
	lastError   *string
	lastResult  map[string]any
	etag        string
	fileVersion int
}

func catalogFile(dataDir string) string {
	return filepath.Join(dataDir, "model-catalog.json")
}

func catalogRawFile(dataDir string) string {
	return filepath.Join(dataDir, "model-catalog-raw.json")
}

// catalogBaseId: baseId do JS.
func catalogBaseId(modelID string) string {
	value := modelID
	if strings.Contains(value, "/") {
		parts := strings.Split(value, "/")
		value = parts[len(parts)-1]
	}
	value = strings.ToLower(value)
	if index := strings.IndexByte(value, ':'); index >= 0 {
		return value[:index]
	}
	return value
}

type ModelCatalogSync struct {
	DataDir string

	// Hooks de teste; default usa fetch/envio reais.
	Fetch   func(ctx context.Context, ifNoneMatch string) (status int, etag string, body []byte, err error)
	Entries func() []map[string]any

	state modelCatalogSyncState
}

func NewModelCatalogSync(dataDir string) *ModelCatalogSync {
	return &ModelCatalogSync{DataDir: dataDir}
}

func modelCatalogWriteAtomic(file string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, contents, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

// catalogSlim: aplicativo enxuto para o add-models (~470KB).
func catalogSlim(catalog map[string]any) map[string]any {
	out := map[string]any{}
	for providerID, rawProvider := range catalog {
		provider, _ := rawProvider.(map[string]any)
		models := map[string]any{}
		rawModels, _ := provider["models"].(map[string]any)
		for modelID, rawModel := range rawModels {
			model, _ := rawModel.(map[string]any)
			input := []any{}
			if modalities, ok := model["modalities"].(map[string]any); ok {
				if rawInput, ok := modalities["input"].([]any); ok {
					for _, item := range rawInput {
						if text, ok := item.(string); ok && text != "text" {
							input = append(input, item)
						}
					}
				}
			}
			entry := map[string]any{"i": input}
			if limit, ok := model["limit"].(map[string]any); ok {
				if context, exists := limit["context"]; exists {
					entry["c"] = context
				}
				if output, exists := limit["output"]; exists {
					entry["o"] = output
				}
			}
			if reasoning, ok := model["reasoning"]; ok {
				entry["r"] = reasoning
			} else {
				entry["r"] = nil
			}
			models[modelID] = entry
		}
		out[providerID] = models
	}
	return out
}

// catalogBuild espelha build(): modalities por provider/modelo + deltas de limites.
func catalogBuild(catalog map[string]any, entries []map[string]any) (map[string]any, map[string]any) {
	localIds := map[string][]string{}
	for _, entry := range entries {
		provider, _ := entry["provider"].(string)
		if provider == "" {
			continue
		}
		upstream := provider
		if alias, ok := catalogAliases[provider]; ok {
			upstream = alias
		}
		list := localIds[upstream]
		if !containsString(list, provider) {
			localIds[upstream] = append(list, provider)
		}
	}

	byProvider := map[string]map[string]any{}
	models := map[string]any{}
	for providerID, rawProvider := range catalog {
		provider, _ := rawProvider.(map[string]any)
		locals := localIds[providerID]
		if len(locals) == 0 {
			locals = []string{providerID}
		}
		modelsByID := map[string]any{}
		seen := map[string]bool{}
		rawModels, _ := provider["models"].(map[string]any)
		ids := make([]string, 0, len(rawModels))
		for modelID := range rawModels {
			ids = append(ids, modelID)
		}
		sort.Strings(ids)
		for _, modelID := range ids {
			model, _ := rawModels[modelID].(map[string]any)
			id := catalogBaseId(modelID)
			if _, exists := modelsByID[id]; !exists {
				modelsByID[id] = model
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			declared := map[string]any{}
			modalities, _ := model["modalities"].(map[string]any)
			rawInput, _ := modalities["input"].([]any)
			for _, input := range rawInput {
				switch text, _ := input.(string); text {
				case "image":
					declared["vision"] = true
				case "pdf":
					declared["pdf"] = true
				case "audio":
					declared["audioInput"] = true
				case "video":
					declared["videoInput"] = true
				}
			}
			if len(declared) == 0 {
				continue
			}
			for _, local := range locals {
				models[local+":"+id] = declared
			}
			if !containsString(locals, providerID) {
				models[providerID+":"+id] = declared
			}
		}
		byProvider[providerID] = modelsByID
	}

	providersBlock := map[string]any{}
	const tolerance = 0.1
	for _, entry := range entries {
		provider, _ := entry["provider"].(string)
		model, _ := entry["model"].(string)
		contextLength := pxpipeNum(entry["contextLength"])
		current, _ := entry["current"].(map[string]any)
		upstream := provider
		if _, exists := catalog[provider]; !exists {
			found := false
			if alias, ok := catalogAliases[provider]; ok {
				if _, exists := catalog[alias]; exists {
					upstream = alias
					found = true
				}
			}
			if !found {
				upstream = ""
			}
		}
		var entryModel map[string]any
		if upstream != "" {
			entryModel, _ = byProvider[upstream][catalogBaseId(model)].(map[string]any)
		}
		if entryModel == nil {
			continue
		}
		limit, _ := entryModel["limit"].(map[string]any)
		context := pxpipeNum(limit["context"])
		output := pxpipeNum(limit["output"])
		currentWindow := pxpipeNum(current["contextWindow"])
		currentOutput := pxpipeNum(current["maxOutput"])
		delta := map[string]any{}
		if context > 0 && contextLength == 0 && math.Abs(context-currentWindow)/currentWindow > tolerance {
			delta["contextWindow"] = context
		}
		if output > 0 && math.Abs(output-currentOutput)/currentOutput > tolerance {
			delta["maxOutput"] = output
		}
		if len(delta) == 0 {
			continue
		}
		block, _ := providersBlock[provider].(map[string]any)
		if block == nil {
			block = map[string]any{}
			providersBlock[provider] = block
		}
		block[model] = delta
	}
	return models, providersBlock
}

// entriesList monta {provider, model, contextLength, current} do snapshot.
// O snapshot consolida os modelos hand-written (models-catalog.json).
func (s *ModelCatalogSync) entriesList() []map[string]any {
	if s.Entries != nil {
		return s.Entries()
	}
	catalog := providers.CatalogModels()
	entries := []map[string]any{}
	for _, model := range catalog.Models {
		caps, _ := model["caps"].(map[string]any)
		entries = append(entries, map[string]any{
			"provider":      model["provider"],
			"model":         model["model"],
			"contextLength": caps["contextWindow"],
			"current":       caps,
		})
	}
	return entries
}

func (s *ModelCatalogSync) fetcher() func(ctx context.Context, ifNoneMatch string) (int, string, []byte, error) {
	if s.Fetch != nil {
		return s.Fetch
	}
	return modelCatalogFetchReal
}

func modelCatalogFetchReal(ctx context.Context, ifNoneMatch string) (int, string, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, modelCatalogFetchTO)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, "GET", modelCatalogURL, nil)
	if err != nil {
		return 0, "", nil, err
	}
	request.Header.Set("accept", "application/json")
	if ifNoneMatch != "" {
		request.Header.Set("if-none-match", ifNoneMatch)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return 0, "", nil, err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 64<<20))
	return response.StatusCode, response.Header.Get("etag"), body, nil
}

type syncError struct{ Msg string }

func (e *syncError) Error() string { return e.Msg }

func (s *ModelCatalogSync) setStateRunning(value bool) bool {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if value && s.state.running {
		return false
	}
	s.state.running = value
	return true
}

// SyncModelCatalog espelha syncModelCatalog: nil → não completou.
func (s *ModelCatalogSync) SyncModelCatalog(ctx context.Context) map[string]any {
	if !s.setStateRunning(true) {
		return nil
	}
	defer s.setStateRunning(false)

	var result map[string]any
	var failure *string
	if err := s.runSync(ctx, &result); err != nil {
		message := err.Error()
		failure = &message
	}
	s.state.mu.Lock()
	if failure != nil {
		s.state.lastError = failure
	} else {
		now := time.Now().UnixMilli()
		s.state.lastSync = &now
		s.state.lastError = nil
		s.state.lastResult = result
	}
	s.state.mu.Unlock()
	if failure != nil {
		return nil
	}
	return result
}

func (s *ModelCatalogSync) runSync(ctx context.Context, result *map[string]any) error {
	s.state.mu.Lock()
	etag := s.state.etag
	fileVersion := s.state.fileVersion
	s.state.mu.Unlock()
	ifNoneMatch := ""
	if etag != "" && fileVersion == modelCatalogVersion {
		ifNoneMatch = etag
	}
	status, newEtag, body, err := s.fetcher()(ctx, ifNoneMatch)
	if err != nil {
		return err
	}
	if status == 304 {
		*result = map[string]any{"status": "unchanged"}
		return nil
	}
	if status < 200 || status >= 300 {
		return &syncError{Msg: fmt.Sprintf("HTTP %d", status)}
	}
	var catalog map[string]any
	if json.Unmarshal(body, &catalog) != nil {
		return &syncError{Msg: "failed to parse upstream catalog"}
	}
	models, providersBlock := catalogBuild(catalog, s.entriesList())
	serialized, err := json.Marshal(map[string]any{
		"v":         modelCatalogVersion,
		"etag":      nilIfEmpty(newEtag),
		"syncedAt":  time.Now().UnixMilli(),
		"models":    models,
		"providers": providersBlock,
	})
	if err != nil {
		return err
	}
	if err := modelCatalogWriteAtomic(catalogFile(s.DataDir), serialized); err != nil {
		return err
	}
	encodedRaw, _ := json.Marshal(catalogSlim(catalog))
	if err := modelCatalogWriteAtomic(catalogRawFile(s.DataDir), encodedRaw); err != nil {
		return err
	}
	s.state.mu.Lock()
	s.state.etag = newEtag
	s.state.fileVersion = modelCatalogVersion
	s.state.mu.Unlock()
	*result = map[string]any{
		"status":    "updated",
		"etag":      nilIfEmpty(newEtag),
		"bytes":     len(serialized),
		"models":    len(models),
		"providers": len(providersBlock),
	}
	return nil
}

func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// SyncState espelha getSyncState().
func (s *ModelCatalogSync) SyncState() map[string]any {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	out := map[string]any{
		"running":     s.state.running,
		"lastSync":    nilOrNilInt(s.state.lastSync),
		"lastError":   nilOrNilString(s.state.lastError),
		"lastResult":  s.state.lastResult,
		"etag":        strOrNil(s.state.etag),
		"fileVersion": nilOrVersion(s.state.fileVersion),
		"file":        catalogFile(s.DataDir),
		"url":         modelCatalogURL,
		"intervalMs":  modelCatalogInterval.Milliseconds(),
	}
	return out
}

func nilOrNilInt(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func nilOrNilString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func strOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nilOrVersion(value int) any {
	if value == 0 {
		return nil
	}
	return value
}

// Get espelha GET /api/models/catalog-sync com o resumo do catálogo.
func (s *ModelCatalogSync) Get(w http.ResponseWriter, r *http.Request) {
	state := s.SyncState()
	syncedAt, models, providerCount, bytes := catalogSummary(s.DataDir)
	catalog := map[string]any{}
	if models >= 0 {
		catalog = map[string]any{
			"syncedAt":  syncedAt,
			"models":    models,
			"providers": providerCount,
			"bytes":     bytes,
		}
	} else {
		catalog = nil
	}
	state["catalog"] = catalog
	nodeReply(w, 200, state)
}

// POST: roda o sync agora; falha → 503 com lastError ou "sync in progress".
func (s *ModelCatalogSync) Post(w http.ResponseWriter, r *http.Request) {
	result := s.SyncModelCatalog(r.Context())
	if result == nil {
		message := "sync in progress"
		s.state.mu.Lock()
		if s.state.running {
			message = "sync in progress"
		} else if s.state.lastError != nil {
			message = *s.state.lastError
		}
		s.state.mu.Unlock()
		nodeReply(w, 503, map[string]any{"error": message})
		return
	}
	nodeReply(w, 200, map[string]any{"success": true, "result": result})
}

// catalogSummary: count de models/providers/bytes do arquivo atual (-1 = nulo).
func catalogSummary(dataDir string) (any, int, int, int64) {
	file := catalogFile(dataDir)
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, -1, -1, 0
	}
	var parsed struct {
		SyncedAt  any            `json:"syncedAt"`
		Models    map[string]any `json:"models"`
		Providers map[string]any `json:"providers"`
	}
	if json.Unmarshal(raw, &parsed) != nil {
		return nil, -1, -1, 0
	}
	info, _ := os.Stat(file)
	var bytes int64
	if info != nil {
		bytes = info.Size()
	}
	return parsed.SyncedAt, len(parsed.Models), len(parsed.Providers), bytes
}

// restoreEtag: resume do arquivo escrito anteriormente.
func (s *ModelCatalogSync) restoreFromFile() {
	raw, err := os.ReadFile(catalogFile(s.DataDir))
	if err != nil {
		return
	}
	var parsed struct {
		Etag *string `json:"etag"`
		V    int     `json:"v"`
	}
	if json.Unmarshal(raw, &parsed) != nil {
		return
	}
	info, statErr := os.Stat(catalogFile(s.DataDir))
	s.state.mu.Lock()
	s.state.etag = strOrEmpty(parsed.Etag)
	s.state.fileVersion = parsed.V
	if statErr == nil {
		mtime := info.ModTime().UnixMilli()
		s.state.lastSync = &mtime
	}
	s.state.mu.Unlock()
}

func strOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// StartTimer espelha startModelCatalogSync: sync automático (env off desliga).
// O guard `timer` do JS vira started; disparos múltiplos não duplicam.
func (s *ModelCatalogSync) StartTimer() {
	if strings.EqualFold(os.Getenv("MODEL_CATALOG_SYNC"), "off") {
		return
	}
	s.state.mu.Lock()
	if s.state.started {
		s.state.mu.Unlock()
		return
	}
	s.state.started = true
	s.state.mu.Unlock()
	s.restoreFromFile()
	var fire func(delay time.Duration)
	fire = func(delay time.Duration) {
		time.AfterFunc(delay, func() {
			result := s.SyncModelCatalog(context.Background())
			next := modelCatalogInterval
			if result == nil {
				next = modelCatalogRetry
			}
			fire(next)
		})
	}
	fire(modelCatalogStart)
}

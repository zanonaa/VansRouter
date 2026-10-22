package handlers

// Porta de open-sse/services/proxyPoolFitness.js (cache de leitura + DB fonte).
import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/zanonaa/VansRouter/backend/internal/database"
)

const PoolUnfitMs = int64(5 * 60 * 1000)

// PoolFitnessEntry em DB e cache; implementa o contrato listProxyPoolFitness.
type poolFitnessEntry struct {
	PoolID string
	Scope  string
	Until  int64
	Reason string
}

// PoolFitnessService guarda o cache read-through; a fonte de verdade é o DB.
type PoolFitnessService struct {
	Store database.ProxyFitness
}

func (s *PoolFitnessService) store() database.ProxyFitness { return s.Store }

var poolFitnessMu sync.Mutex
var poolFitnessCache = map[string]map[string]poolFitnessEntry{}

func setPoolFitnessCached(poolID string, entries []poolFitnessEntry) {
	if len(entries) > 0 {
		byScope := map[string]poolFitnessEntry{}
		for _, entry := range entries {
			byScope[entry.Scope] = poolFitnessEntry{Until: entry.Until, Reason: entry.Reason}
		}
		poolFitnessCache[poolID] = byScope
	} else {
		delete(poolFitnessCache, poolID)
	}
}

func entriesFromCached(poolID string) []poolFitnessEntry {
	byScope := poolFitnessCache[poolID]
	if byScope == nil {
		return nil
	}
	entries := make([]poolFitnessEntry, 0, len(byScope))
	for scope, entry := range byScope {
		entries = append(entries, poolFitnessEntry{PoolID: poolID, Scope: scope, Until: entry.Until, Reason: entry.Reason})
	}
	return entries
}

func updateCachedFitness(poolID, scope string, until int64, reason string) {
	byScope := poolFitnessCache[poolID]
	if byScope == nil {
		byScope = map[string]poolFitnessEntry{}
	}
	byScope[scope] = poolFitnessEntry{Until: until, Reason: reason}
	poolFitnessCache[poolID] = byScope
}

// LoadPoolFitness espelha loadPoolFitness (read-through + purge de expirados).
func (s *PoolFitnessService) LoadPoolFitness(ctx context.Context, poolID string) {
	if poolID == "" {
		return
	}
	entries, err := s.store().List(ctx, &poolID)
	if err != nil {
		return // fail-open
	}
	norm := make([]poolFitnessEntry, 0, len(entries))
	for _, entry := range entries {
		norm = append(norm, poolFitnessEntry{PoolID: entry.PoolID, Scope: entry.Scope, Until: int64(entry.Until), Reason: entry.Reason})
	}
	now := time.Now().UnixMilli()
	var active, expired []poolFitnessEntry
	for _, entry := range norm {
		if entry.Until > now {
			active = append(active, entry)
		} else {
			expired = append(expired, entry)
		}
	}
	poolFitnessMu.Lock()
	setPoolFitnessCached(poolID, active)
	poolFitnessMu.Unlock()
	for _, entry := range expired {
		_, _ = s.store().Delete(ctx, entry.PoolID, entry.Scope)
	}
}

// MarkPoolUnfit espelha markPoolUnfit.
func (s *PoolFitnessService) MarkPoolUnfit(ctx context.Context, poolID, scope string, until *int64, reason string) bool {
	if poolID == "" || scope == "" || until == nil {
		return false
	}
	if _, err := s.store().Upsert(ctx, poolID, scope, float64(*until), reason); err != nil {
		return false
	}
	poolFitnessMu.Lock()
	updateCachedFitness(poolID, scope, *until, reason)
	poolFitnessMu.Unlock()
	return true
}

// ClearPoolUnfit espelha clearPoolUnfit.
func (s *PoolFitnessService) ClearPoolUnfit(ctx context.Context, poolID, scope string) bool {
	if poolID == "" || scope == "" {
		return false
	}
	exists, err := s.store().Delete(ctx, poolID, scope)
	_ = exists
	if err != nil {
		return false
	}
	poolFitnessMu.Lock()
	byScope := poolFitnessCache[poolID]
	if byScope != nil {
		delete(byScope, scope)
		if len(byScope) == 0 {
			delete(poolFitnessCache, poolID)
		}
	}
	poolFitnessMu.Unlock()
	return true
}

func providerWildcardScope(scope string) string {
	sep := strings.Index(scope, "::")
	if sep < 0 {
		return ""
	}
	return scope[:sep] + "::*"
}

// IsPoolFit espelha isPoolFit.
func (s *PoolFitnessService) IsPoolFit(poolID, scope string, now int64) bool {
	if poolID == "" {
		return true
	}
	poolFitnessMu.Lock()
	defer poolFitnessMu.Unlock()
	byScope := poolFitnessCache[poolID]
	if byScope == nil {
		return true
	}
	for _, key := range []string{scope, providerWildcardScope(scope)} {
		if key == "" {
			continue
		}
		entry, ok := byScope[key]
		if !ok {
			continue
		}
		if entry.Until <= now {
			delete(byScope, key)
			if len(byScope) == 0 {
				delete(poolFitnessCache, poolID)
			}
			continue
		}
		return false
	}
	return true
}

// FitPoolIds espelha fitPoolIds.
func (s *PoolFitnessService) FitPoolIds(poolIDs []string, scope string, now int64) []string {
	if poolIDs == nil {
		return nil
	}
	out := []string{}
	for _, id := range poolIDs {
		if s.IsPoolFit(id, scope, now) {
			out = append(out, id)
		}
	}
	return out
}

// ClearAllPoolUnfit espelha clearAllPoolUnfit (provider vazio = todos).
func (s *PoolFitnessService) ClearAllPoolUnfit(ctx context.Context, provider string) bool {
	if _, err := s.store().Clear(ctx, provider); err != nil {
		return false
	}
	poolFitnessMu.Lock()
	if provider == "" {
		poolFitnessCache = map[string]map[string]poolFitnessEntry{}
	} else {
		prefix := provider + "::"
		for poolID, byScope := range poolFitnessCache {
			for scope := range byScope {
				if strings.HasPrefix(scope, prefix) {
					delete(byScope, scope)
				}
			}
			if len(byScope) == 0 {
				delete(poolFitnessCache, poolID)
			}
		}
	}
	poolFitnessMu.Unlock()
	return true
}

// ResetPoolFitness espelha resetPoolFitness.
func (s *PoolFitnessService) ResetPoolFitness(ctx context.Context) bool {
	if _, err := s.store().Clear(ctx, ""); err != nil {
		return false
	}
	poolFitnessMu.Lock()
	poolFitnessCache = map[string]map[string]poolFitnessEntry{}
	poolFitnessMu.Unlock()
	return true
}

// PruneExpired espelha pruneExpired (todas as entradas).
func (s *PoolFitnessService) PruneExpired(ctx context.Context, now int64) int {
	entries, err := s.store().List(ctx, nil)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if int64(entry.Until) > now {
			continue
		}
		if _, err := s.store().Delete(ctx, entry.PoolID, entry.Scope); err == nil {
			count++
		}
		poolFitnessMu.Lock()
		byScope := poolFitnessCache[entry.PoolID]
		if byScope != nil {
			delete(byScope, entry.Scope)
			if len(byScope) == 0 {
				delete(poolFitnessCache, entry.PoolID)
			}
		}
		poolFitnessMu.Unlock()
	}
	return count
}

// Snapshot espelha poolFitnessSnapshot: {poolId: {scope: {until, reason}}}.
func (s *PoolFitnessService) Snapshot(ctx context.Context, now int64) map[string]any {
	entries, err := s.store().List(ctx, nil)
	if err != nil {
		return nil
	}
	out := map[string]any{}
	for _, entry := range entries {
		poolEntry := poolFitnessEntry{PoolID: entry.PoolID, Scope: entry.Scope, Until: int64(entry.Until), Reason: entry.Reason}
		if poolEntry.Until <= now {
			_, _ = s.store().Delete(ctx, entry.PoolID, entry.Scope)
			continue
		}
		byScope, _ := out[entry.PoolID].(map[string]any)
		if byScope == nil {
			byScope = map[string]any{}
			out[entry.PoolID] = byScope
		}
		byScope[entry.Scope] = map[string]any{"until": entry.Until, "reason": entry.Reason}
		poolFitnessMu.Lock()
		merged := entriesFromCached(entry.PoolID)
		merged = append(merged, poolEntry)
		setPoolFitnessCached(entry.PoolID, merged)
		poolFitnessMu.Unlock()
	}
	return out
}
